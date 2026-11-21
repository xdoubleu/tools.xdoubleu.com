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
	// defaultNixStoreMax is the store size above which a session collects garbage.
	defaultNixStoreMax int64 = 3 << 30
	// kib is the unit of `du -sk`, which macOS and Linux share (`-b` is Linux only).
	kib int64 = 1024
)

// flakeInputs lists a flake.lock's direct GitHub inputs as "name owner repo rev".
const flakeInputs = `.nodes as $n | $n.root.inputs | to_entries[]
  | select(.value | type == "string") | .key as $k | $n[.value].locked
  | select(.type == "github") | "\($k) \(.owner) \(.repo) \(.rev)"`

// nixScript is the part of a session's setup that enters the checkout's flake dev
// shell, or "" when NixStore is unset. Inputs and binaries come through the egress
// proxy's read-only nixsrc and nixcache routes (docs/adr-0055); locked GitHub
// inputs are fetched as tarballs by their pinned revision.
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
	var b strings.Builder
	b.WriteString("[ -e " + root + "/var/nix/profiles/default ] || cp -a " +
		seed + "/. " + root + "/\n")
	b.WriteString("export PATH=" + root + "/var/nix/profiles/default/bin:$PATH\n")
	// No sandbox or build users: the VM is the sandbox, and it holds no capabilities.
	b.WriteString("export NIX_CONFIG='experimental-features = nix-command flakes\n" +
		"sandbox = false\nbuild-users-group =\nsubstituters = " + substituters + "'\n")
	b.WriteString("[ \"$(du -sk " + root + "/store | cut -f1)\" -le " +
		strconv.FormatInt((limit+kib-1)/kib, 10) + " ] || nix store gc >&2 || true\n")
	b.WriteString("if [ -f flake.nix ]; then\n  set -- --command \"$@\"\n")
	if proxyURL != "" {
		b.WriteString("  if [ -f flake.lock ]; then\n    list=$(jq -r '" + flakeInputs +
			"' flake.lock)\n    while read -r in owner repo rev; do\n" +
			"      [ -n \"$in\" ] || continue\n" +
			"      set -- --override-input \"$in\" " +
			"\"tarball+" + proxyURL + "/nixsrc/$owner/$repo/tar.gz/$rev\" \"$@\"\n" +
			"    done <<EOF\n$list\nEOF\n  fi\n")
	}
	b.WriteString("  set -- nix develop --no-write-lock-file \"$@\"\nfi\n")
	return b.String()
}
