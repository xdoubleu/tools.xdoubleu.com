package vm

import (
	"strconv"
	"strings"
)

const (
	// nixMount is where NixStore is mounted: the shared /nix of every session.
	nixMount = "/nix"
	// nixSeed is the Nix install the image keeps outside /nix, which the mount
	// hides; the first session on an empty store copies it in.
	nixSeed = "/opt/nix-seed"
	// defaultNixStoreMax is the store size above which a session collects garbage,
	// well above the working set so a collection is rare.
	defaultNixStoreMax int64 = 16 << 30
	// kib is the unit of `df -Pk`, which macOS and Linux share.
	kib int64 = 1024
	// nixGCLog is where, under the store's root, the garbage collection logs.
	nixGCLog = "var/log/wardcase-gc.log"
	// nixLock is the lock, under the store's root, that serialises the seed, the
	// collection and profile writes across the VMs sharing the store.
	nixLock = "var/wardcase.lock"
	// nixLockStaleMinutes is when a lock its holder never released is broken, and
	// nixLockWait how many seconds a session waits before failing.
	nixLockStaleMinutes = "20"
	nixLockWait         = "1500"
	// nixDevEnvs is where, under the store's root, each repo's dev-shell
	// environment is cached by the hash of its flake.nix and flake.lock.
	nixDevEnvs = "var/wardcase/devenv"
)

// flakeInputs lists a flake.lock's direct GitHub inputs as "name owner repo rev".
const flakeInputs = `.nodes as $n | $n.root.inputs | to_entries[]
  | select(.value | type == "string") | .key as $k | $n[.value].locked
  | select(.type == "github") | "\($k) \(.owner) \(.repo) \(.rev)"`

// nixScript is the part of a session's setup that enters the checkout's flake dev
// shell, or "" when NixStore is unset. Inputs and binaries come through the egress
// proxy's read-only nixsrc and nixcache routes (docs/adr-0055).
func (s *ContainerSandbox) nixScript(proxyURL string) string {
	if s.NixStore == "" {
		return ""
	}
	return s.nixScriptAt(nixMount, nixSeed, proxyURL)
}

func (s *ContainerSandbox) nixScriptAt(root, seed, proxyURL string) string {
	limit := s.NixStoreMax
	if limit <= 0 {
		limit = defaultNixStoreMax
	}
	substituters := ""
	if proxyURL != "" {
		substituters = proxyURL + "/nixcache/"
	}
	lock := root + "/" + nixLock
	var b strings.Builder
	// An atomic mkdir is the lock: flock is not on macOS, where the tests run.
	b.WriteString("nix_lock() {\n  n=0\n  until mkdir " + lock + " 2>/dev/null; do\n" +
		"    [ -z \"$(find " + lock + " -prune -mmin +" + nixLockStaleMinutes +
		" 2>/dev/null)\" ] || rmdir " + lock + " 2>/dev/null\n" +
		"    n=$((n + 1))\n    [ \"$n\" -lt " + nixLockWait + " ] || " +
		"{ echo 'wardcase: cannot lock " + root + "' >&2; exit 1; }\n" +
		"    sleep 1\n  done\n}\nnix_unlock() { rmdir " + lock + " 2>/dev/null; }\n")
	b.WriteString("mkdir -p " + root + "/var/log 2>/dev/null\nnix_lock\n")
	// Seed only a store with nothing in it: a populated one without its profile
	// is a dead or broken mount, and copying over it makes things worse.
	// No -p: the mount point's own times and owner cannot be set, and would fail.
	b.WriteString("if [ ! -e " + root + "/var/nix/profiles/default ]; then\n" +
		"  if [ -n \"$(ls -A " + root + "/store 2>/dev/null)\" ]; then\n" +
		"    nix_unlock\n    echo 'wardcase: /nix is not empty but has no default " +
		"profile; the store mount is broken' >&2\n    exit 1\n  fi\n" +
		"  cp -RP " + seed + "/. " + root + "/ || { nix_unlock; echo 'wardcase: seeding " +
		root + " from " + seed + " failed' >&2; exit 1; }\nfi\n")
	b.WriteString("export PATH=" + root + "/var/nix/profiles/default/bin:$PATH\n")
	// No sandbox or build users: the VM is the sandbox, and it holds no capabilities.
	b.WriteString("export NIX_CONFIG='experimental-features = nix-command flakes\n" +
		"sandbox = false\nbuild-users-group =\nsubstituters = " + substituters + "'\n")
	// The collection's output goes to a log in the store: on the job's stderr it
	// would lead every failure message ahead of the command that failed. The
	// volume holds only the store, so its used space is the store's size: a `du`
	// would open every inode, and the host keeps a descriptor for each.
	b.WriteString("used=$(df -Pk " + root +
		" 2>/dev/null | awk 'NR == 2 { print $3 }')\n" +
		"{ [ \"${used:-0}\" -le " + strconv.FormatInt((limit+kib-1)/kib, 10) +
		" ] || nix store gc || true; } >>" + root + "/" + nixGCLog + " 2>&1\n")
	b.WriteString("if [ -f flake.nix ]; then\n")
	// The profile roots the dev shell against the store's GC, one per repo with
	// its older generations wiped.
	b.WriteString("  dev=$(git config --get remote.origin.url 2>/dev/null | " +
		"sed 's|.*/||; s|\\.git$||')\n" +
		"  [ -n \"$dev\" ] || dev=$(basename \"$PWD\")\n" +
		"  profile=" + root + "/var/nix/profiles/devshell-$dev\n")
	// Evaluating the flake unpacks and reads all of nixpkgs, so the environment
	// is cached by the flake's hash and only a changed flake evaluates it again.
	b.WriteString("  sum=$(cat flake.nix flake.lock 2>/dev/null | " +
		"{ sha256sum 2>/dev/null || shasum -a 256; } | cut -c1-32)\n" +
		"  devenvs=" + root + "/" + nixDevEnvs + "\n" +
		"  devenv=$devenvs/$dev-$sum.sh\n" +
		"  if [ ! -s \"$devenv\" ] || [ ! -e \"$profile\" ]; then\n" +
		"    mkdir -p \"$devenvs\"\n" +
		"    nix profile wipe-history --profile \"$profile\" >>" + root + "/" +
		nixGCLog + " 2>&1 || true\n")
	prefix := ""
	if proxyURL != "" {
		// The proxy tunnels fixed-output fetches (a tool built from source) to
		// its fetch hosts when nix is given it.
		prefix = "env https_proxy=" + proxyURL + " "
	}
	b.WriteString("    if ( set --\n")
	if proxyURL != "" {
		// Locked GitHub inputs come as tarballs by their pinned revision.
		b.WriteString("      if [ -f flake.lock ]; then\n        list=$(jq -r '" +
			flakeInputs + "' flake.lock)\n        while read -r in owner repo rev; do\n" +
			"          [ -n \"$in\" ] || continue\n" +
			"          set -- \"$@\" --override-input \"$in\" " +
			"\"tarball+" + proxyURL + "/nixsrc/$owner/$repo/tar.gz/$rev\"\n" +
			"        done <<EOF\n$list\nEOF\n      fi\n")
	}
	b.WriteString("      " + prefix + "nix print-dev-env --profile \"$profile\" " +
		"--no-write-lock-file \"$@\"\n" +
		"    ) >\"$devenv.tmp\"; then\n" +
		"      rm -f \"$devenvs/$dev\"-*.sh\n      mv \"$devenv.tmp\" \"$devenv\"\n" +
		"    else\n      rm -f \"$devenv.tmp\"\n      nix_unlock\n" +
		"      echo 'wardcase: building the flake dev shell failed' >&2\n      exit 1\n" +
		"    fi\n  fi\n")
	// The dev shell's own command runs without the proxy variable.
	b.WriteString("fi\nnix_unlock\nif [ -f flake.nix ]; then\n" +
		"  set -- bash -c '. \"$0\"; unset https_proxy; exec \"$@\"' " +
		"\"$devenv\" \"$@\"\nfi\n")
	return b.String()
}
