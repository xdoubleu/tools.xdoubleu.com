// Command wardcase runs an agent harness (Claude Code, OpenCode, Codex, Gemini) in the job
// VM on the current checkout, so any ADE or terminal gets the same sandbox.
// Experimental.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

const usage = `wardcase runs an agent harness in a sealed VM on this checkout.
(experimental)

  wardcase [--in-place] [claude|opencode|codex|gemini] [args...]
                          interactive; args go to the harness
  wardcase --print [claude|opencode|codex|gemini] [prompt]
                          headless; the prompt, else stdin
  wardcase secret set <anthropic|openrouter|openai|gemini>
                          reads the value from stdin
  wardcase acp [opencode|codex|gemini]
                          an ACP agent on stdio, for an ADE
  wardcase config                                    prints the config path and settings
  wardcase mcp login <name>                          signs a configured MCP server in
  wardcase mcp list                                  the configured MCP servers
  wardcase mcp logout <name>                         forgets a server's sign-in

Each run gets its own git worktree on a new branch cut from HEAD; on exit wardcase
says what is left in it and removes it when nothing is. --in-place runs in the
current directory instead. --resume goes on in the checkout's latest worktree.

wardcase acp serves one ACP session over its own stdio, in a worktree of the directory
the ADE names, and allows every tool call in the VM: the sandbox is the boundary.

MCP servers are the config's [[mcp]] entries (name, url, jobs). They are signed in
here, on the Mac, with OAuth; the VM reaches them through the proxy, which adds the
token, and never holds one.

Without a harness the first in the config's order runs. At a usage limit the next
one takes over, as the config's overflow (ask, auto, off) allows.
`

// Version is the release wardcase is built at, stamped with
// -ldflags "-X main.Version=...". The base image of the same version is the only
// image wardcase runs.
//
//nolint:gochecknoglobals // set by the linker
var Version = "dev"

// ImageRepo is where each release publishes its base image.
const ImageRepo = "ghcr.io/xdoubleu/wardcase"

// ImageRef is the base image of one release.
func ImageRef(version string) string { return ImageRepo + ":" + version }

// stateDirPerm is the mode of wardcase's own state directories.
const stateDirPerm = 0o750

// exitUsage is the exit code for a malformed command line.
const exitUsage = 2

// host is the Mac's container CLI and keychain; tests pass fakes.
type host struct {
	exec    vm.Execer
	secrets ward.Secrets
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	h := host{exec: vm.OSExecer{BinDirs: nil}, secrets: ward.NewKeychain()}
	code := run(ctx, h, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(
	ctx context.Context, h host, args []string, in *os.File, out, errOut io.Writer,
) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			fmt.Fprint(out, usage)
			return 0
		case "secret":
			return secret(h.secrets, args[1:], in, out, errOut)
		case "config":
			return showConfig(home, out, errOut)
		case "mcp":
			return mcp(ctx, h, home, args[1:], out, errOut)
		case "acp":
			return acpCommand(ctx, h, home, args[1:], in, out, errOut)
		}
	}
	code, err := start(ctx, h, home, args, in, out, errOut)
	if err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	return code
}

func secret(
	store ward.Secrets, args []string, in io.Reader, out, errOut io.Writer,
) int {
	if len(args) != 2 || args[0] != "set" || !ward.ValidSecret(args[1]) {
		fmt.Fprintln(errOut, "usage: wardcase secret set <anthropic|openrouter|openai|gemini>")
		return exitUsage
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	value := strings.TrimSpace(line)
	if value == "" || (err != nil && !errors.Is(err, io.EOF)) {
		fmt.Fprintln(errOut,
			"wardcase: pipe the value in, e.g. pbpaste | wardcase secret set", args[1])
		return 1
	}
	if err = store.Set(args[1], value); err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	fmt.Fprintf(out, "Stored the %s secret in the keychain.\n", args[1])
	return 0
}

func showConfig(home string, out, errOut io.Writer) int {
	path := ward.ConfigPath(home)
	cfg, err := ward.LoadConfig(path)
	if err != nil {
		fmt.Fprintln(errOut, "wardcase:", err)
		return 1
	}
	fmt.Fprintf(out, "%s\norder = %q\noverflow = %q\nopencode.model = %q\n",
		path, cfg.Order, cfg.Overflow, cfg.OpenCode.Model)
	return 0
}

func start(
	ctx context.Context,
	h host,
	home string,
	args []string,
	in *os.File,
	out, errOut io.Writer,
) (int, error) {
	cfg, err := ward.LoadConfig(ward.ConfigPath(home))
	if err != nil {
		return 1, err
	}
	req := ward.Request{Dir: "", Harness: "", Args: nil, Print: false, InPlace: false}
	if len(args) > 0 && args[0] == "--print" {
		req.Print, args = true, args[1:]
	}
	if len(args) > 0 && args[0] == "--in-place" {
		req.InPlace, args = true, args[1:]
	}
	if len(args) > 0 && ward.ValidHarness(args[0]) {
		req.Harness, args = args[0], args[1:]
	}
	req.Args = args
	if req.Dir, err = os.Getwd(); err != nil {
		return 1, fmt.Errorf("working directory: %w", err)
	}
	r, err := newRunner(ctx, h, home, cfg, in, out, errOut)
	if err != nil {
		return 1, err
	}
	return r.Run(ctx, req)
}

// newRunner checks the container CLI and image and returns a Runner on them.
func newRunner(
	ctx context.Context,
	h host,
	home string,
	cfg ward.Config,
	in *os.File,
	out, errOut io.Writer,
) (*ward.Runner, error) {
	execer := h.exec
	box := &vm.ContainerSandbox{
		Image: ImageRef(Version), Exec: execer, Proxy: nil, ProxyPort: vm.EgressProxyPort,
		GoModCache: "", NPMCache: "",
		NixStore:    filepath.Join(home, ".local", "state", "wardcase", "nix"),
		NixStoreMax: 0, CPUs: "", Memory: "", TempDir: "",
	}
	if err := box.Check(ctx); err != nil {
		return nil, err
	}
	if err := ensureImage(ctx, execer, box.Image, errOut); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(box.NixStore, stateDirPerm); err != nil {
		return nil, fmt.Errorf("nix store directory: %w", err)
	}
	terminal := isTerminal(in)
	return &ward.Runner{
		Box: box, Secrets: h.secrets, Config: cfg,
		StateDir: filepath.Join(home, ".local", "state", "wardcase"), TempDir: "",
		Git: ward.HostGit, GitEnv: nil, GH: ward.HostGH, NewProxy: newProxy, In: in,
		Out: out, Notes: errOut,
		Terminal: terminal, Ask: ask(in, errOut), Now: nil,
	}, nil
}

func newProxy(cfg vm.EgressConfig) (vm.ProxyListener, error) {
	return vm.NewEgressProxy(cfg)
}

// ensureImage pulls image unless the container CLI already has it.
func ensureImage(
	ctx context.Context, execer vm.Execer, image string, notes io.Writer,
) error {
	var out bytes.Buffer
	code, err := execer.Exec(ctx, []string{vm.ContainerCLI, "image", "inspect", image},
		nil, &out)
	if err == nil && code == 0 {
		return nil
	}
	fmt.Fprintf(notes, "wardcase: pulling %s\n", image)
	out.Reset()
	code, err = execer.Exec(ctx, []string{vm.ContainerCLI, "image", "pull", image},
		nil, &out)
	if err != nil || code != 0 {
		return fmt.Errorf("pull %s: %w", image, errors.Join(err,
			fmt.Errorf("exit %d: %s", code, strings.TrimSpace(out.String()))))
	}
	return nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// ask reads a yes/no answer from the terminal; an empty answer is yes.
func ask(in io.Reader, errOut io.Writer) func(string) bool {
	return func(question string) bool {
		fmt.Fprintf(errOut, "%s [Y/n] ", question)
		line, _ := bufio.NewReader(in).ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		return answer == "" || answer == "y" || answer == "yes"
	}
}
