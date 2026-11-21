// Command wardcase runs an agent harness (Claude Code, OpenCode, Codex) in the job
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
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

const usage = `wardcase runs an agent harness in a sealed VM on this checkout.
(experimental)

  wardcase [claude|opencode|codex] [args...]         interactive; args go to the harness
  wardcase --print [claude|opencode|codex] [prompt]  headless; the prompt, else stdin
  wardcase secret set <anthropic|openrouter|openai>  reads the value from stdin
  wardcase config                                    prints the config path and settings

Without a harness the first in the config's order runs. At a usage limit the next
one takes over, as the config's overflow (ask, auto, off) allows.
`

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
		fmt.Fprintln(errOut, "usage: wardcase secret set <anthropic|openrouter|openai>")
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
	fmt.Fprintf(out, "%s\norder = %q\noverflow = %q\nimage = %q\nopencode.model = %q\n",
		path, cfg.Order, cfg.Overflow, cfg.Image, cfg.OpenCode.Model)
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
	req := ward.Request{Dir: "", Harness: "", Args: nil, Print: false}
	if len(args) > 0 && args[0] == "--print" {
		req.Print, args = true, args[1:]
	}
	if len(args) > 0 && ward.ValidHarness(args[0]) {
		req.Harness, args = args[0], args[1:]
	}
	req.Args = args
	if req.Dir, err = os.Getwd(); err != nil {
		return 1, fmt.Errorf("working directory: %w", err)
	}
	execer := h.exec
	if cfg.Image == "" {
		if cfg.Image, err = newestJobImage(ctx, execer); err != nil {
			return 1, err
		}
	}
	box := &vm.ContainerSandbox{
		Image: cfg.Image, Exec: execer, Proxy: nil, ProxyPort: vm.EgressProxyPort,
		GoModCache: "", NPMCache: "", CPUs: "", Memory: "", TempDir: "",
	}
	if err = box.Check(ctx); err != nil {
		return 1, err
	}
	terminal := isTerminal(in)
	r := &ward.Runner{
		Box: box, Secrets: h.secrets, Config: cfg,
		StateDir: filepath.Join(home, ".local", "state", "wardcase"), TempDir: "",
		Git: hostGit, NewProxy: newProxy, In: in, Out: out, Notes: errOut,
		Terminal: terminal, Ask: ask(in, errOut), Now: nil,
	}
	return r.Run(ctx, req)
}

func newProxy(cfg vm.EgressConfig) (vm.ProxyListener, error) {
	return vm.NewEgressProxy(cfg)
}

func hostGit(ctx context.Context, dir string, args ...string) (string, error) {
	//nolint:gosec // git with wardcase's own fixed arguments
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

// newestJobImage is the first factory-job image `container image list` shows.
func newestJobImage(ctx context.Context, execer vm.Execer) (string, error) {
	var listing bytes.Buffer
	argv := []string{vm.ContainerCLI, "image", "list"}
	code, err := execer.Exec(ctx, argv, nil, &listing)
	if err != nil || code != 0 {
		return "", fmt.Errorf("list the container images: %w", errors.Join(err,
			fmt.Errorf("exit %d", code)))
	}
	return jobImageFrom(listing.String())
}

func jobImageFrom(listing string) (string, error) {
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "factory-job" {
			return "factory-job:" + fields[1], nil
		}
	}
	return "", errors.New("no factory-job image: build one with `make image` in " +
		"factory-runner, or set image in the wardcase config")
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
