package ward

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tools.xdoubleu.com/wardcase/vm"
)

const (
	stateDirPerm = 0o700
	handoffPerm  = 0o600
	checkoutKey  = 16
)

// GitFunc runs git in dir and returns its standard output.
type GitFunc func(ctx context.Context, dir string, args ...string) (string, error)

// interactive is a harness that can also run on a terminal.
type interactive interface {
	vm.Harness
	Interactive(args []string) []string
}

// Runner runs a harness in the job VM on a checkout.
type Runner struct {
	Box     vm.SessionSandbox
	Secrets Secrets
	Config  Config
	// StateDir keeps the harnesses' sessions, one directory per checkout.
	StateDir string
	// TempDir holds each run's outcome directory; empty means os.TempDir().
	TempDir  string
	Git      GitFunc
	NewProxy func(vm.EgressConfig) (vm.ProxyListener, error)
	In       io.Reader
	Out      io.Writer
	// Notes receives wardcase's own messages.
	Notes io.Writer
	// Terminal says In and Out are the user's terminal.
	Terminal bool
	// Ask puts a yes/no question to the user; nil means no one can answer.
	Ask func(question string) bool
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Request is one wardcase invocation.
type Request struct {
	// Dir is the directory wardcase was started in.
	Dir string
	// Harness is the one to start with; empty means the first in Config.Order.
	Harness string
	// Args go to the harness, or are the prompt when Print is set.
	Args []string
	// Print runs the harness headless on a prompt instead of on the terminal.
	Print bool
}

type outcome struct {
	code    int
	limited bool
	resets  time.Time
}

// Run runs the request, going on to the next harness at a usage limit as the
// config allows, and returns the last harness's exit code.
func (r *Runner) Run(ctx context.Context, req Request) (int, error) {
	name := req.Harness
	if name == "" {
		name = r.Config.Order[0]
	}
	checkout, mounts := r.workspace(ctx, req.Dir)
	since, _ := r.Git(ctx, checkout, "rev-parse", "HEAD")
	since = strings.TrimSpace(since)
	args, prompt := req.Args, ""
	if req.Print {
		var err error
		if prompt, err = r.prompt(req.Args); err != nil {
			return -1, err
		}
	}
	task := prompt
	for {
		res, err := r.runOnce(ctx, name, checkout, mounts, args, prompt, req.Print)
		if err != nil {
			return -1, err
		}
		next := r.Config.next(name)
		if !res.limited || next == "" || !r.mayOverflow(name, next, res.resets, req.Print) {
			return res.code, nil
		}
		if err = r.writeHandoff(ctx, checkout, since, name, next); err != nil {
			return -1, err
		}
		fmt.Fprintf(r.Notes, "wardcase: continuing in %s\n", next)
		if req.Print {
			prompt = handoffPrompt() + "\n\nThe task it was given:\n\n" + task
		} else {
			args = promptArgs(next)
		}
		name = next
	}
}

// prompt is the headless run's prompt: the arguments, else standard input.
func (r *Runner) prompt(args []string) (string, error) {
	if p := strings.TrimSpace(strings.Join(args, " ")); p != "" {
		return p, nil
	}
	in, err := io.ReadAll(r.In)
	if err != nil {
		return "", fmt.Errorf("read the prompt: %w", err)
	}
	if strings.TrimSpace(string(in)) == "" {
		return "", errors.New("--print needs a prompt, as arguments or on standard input")
	}
	return string(in), nil
}

// workspace is the directory mounted at /work, the repository's top level when it
// is one so git works, and the worktree's shared git directory when that lies
// outside it.
func (r *Runner) workspace(ctx context.Context, dir string) (string, []vm.Mount) {
	top, err := r.Git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(top) == "" {
		return dir, nil
	}
	top = strings.TrimSpace(top)
	common, err := r.Git(ctx, top, "rev-parse", "--path-format=absolute",
		"--git-common-dir")
	common = filepath.Clean(strings.TrimSpace(common))
	if err != nil || common == "." || strings.HasPrefix(common+"/", top+"/") {
		return top, nil
	}
	return top, []vm.Mount{{Source: common, Target: common, ReadOnly: false}}
}

func (r *Runner) mayOverflow(from, to string, resets time.Time, headless bool) bool {
	when := ""
	if !resets.IsZero() {
		when = " (it resets " + resets.Local().Format("Mon 15:04") + ")"
	}
	switch r.Config.Overflow {
	case OverflowAuto:
		return true
	case OverflowAsk:
		if headless || !r.Terminal || r.Ask == nil {
			fmt.Fprintf(r.Notes, "wardcase: %s hit its usage limit%s; overflow is ask and "+
				"no one can answer here, so it stops\n", from, when)
			return false
		}
		return r.Ask(fmt.Sprintf("%s hit its usage limit%s. Continue in %s?", from, when, to))
	default:
		fmt.Fprintf(r.Notes, "wardcase: %s hit its usage limit%s\n", from, when)
		return false
	}
}

func promptArgs(name string) []string {
	if name == OpenCode {
		return []string{"--prompt", handoffPrompt()}
	}
	return []string{handoffPrompt()}
}

func (r *Runner) writeHandoff(
	ctx context.Context,
	checkout, since, from, to string,
) error {
	transcript := ""
	if from == Claude {
		transcript = compactTranscript(r.sessionDir(checkout, from))
	}
	text := handoff(from, to, gitState(ctx, r.Git, checkout, since), transcript)
	dir := r.sessionDir(checkout, to)
	if err := os.MkdirAll(dir, stateDirPerm); err != nil {
		return fmt.Errorf("session directory: %w", err)
	}
	path := filepath.Join(dir, HandoffFile)
	if err := os.WriteFile(path, []byte(text), handoffPerm); err != nil {
		return fmt.Errorf("write the handoff: %w", err)
	}
	return nil
}

// sessionDir keeps a harness's sessions for one checkout, so --resume works.
func (r *Runner) sessionDir(checkout, name string) string {
	sum := sha256.Sum256([]byte(checkout))
	return filepath.Join(r.StateDir, hex.EncodeToString(sum[:])[:checkoutKey], name)
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) runOnce(
	ctx context.Context,
	name, checkout string,
	mounts []vm.Mount,
	args []string,
	prompt string,
	headless bool,
) (outcome, error) {
	h, env, egress, err := r.harness(name)
	if err != nil {
		return outcome{}, err
	}
	sessionDir := r.sessionDir(checkout, name)
	if err = os.MkdirAll(sessionDir, stateDirPerm); err != nil {
		return outcome{}, fmt.Errorf("session directory: %w", err)
	}
	outcomeDir, err := os.MkdirTemp(r.TempDir, "wardcase-outcome-")
	if err != nil {
		return outcome{}, fmt.Errorf("outcome directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(outcomeDir) }()
	maps(env, h.SessionEnv(vm.SandboxSessionDir))
	env["FACTORY_JOB"] = "wardcase-" + randomID()
	env[vm.MCPEnv] = vm.MCPOff
	proxy, err := r.NewProxy(egress)
	if err != nil {
		return outcome{}, fmt.Errorf("the egress proxy: %w", err)
	}
	spec := vm.SandboxSpec{
		CloneDir: checkout, OutcomeDir: outcomeDir, SessionDir: sessionDir,
		Env: env, Command: nil, Proxy: proxy, Stdin: "", Log: nil,
		Mounts: mounts, SkipInstall: true,
	}
	session, err := r.Box.Open(ctx, spec)
	if err != nil {
		return outcome{}, fmt.Errorf("start the VM: %w", err)
	}
	defer session.Close()

	started := r.now()
	if headless {
		w := &limitWriter{h: h, out: r.Out, buf: nil, limited: false, resets: time.Time{}}
		code, execErr := session.Exec(ctx, h.Command(), prompt, w)
		w.flush()
		return outcome{code: code, limited: w.limited, resets: w.resets}, execErr
	}
	attacher, ok := session.(vm.Attacher)
	if !ok {
		return outcome{}, errors.New("this sandbox cannot attach a terminal")
	}
	code, err := attacher.Attach(ctx, h.Interactive(args), r.In, r.Out, r.Terminal)
	if err != nil || name != Claude {
		return outcome{code: code, limited: false, resets: time.Time{}}, err
	}
	limited, resets := false, time.Time{}
	if t := latestTranscript(sessionDir); t != "" {
		if info, statErr := os.Stat(t); statErr == nil && !info.ModTime().Before(started) {
			limited, resets = ClaudeLimited(sessionDir)
		}
	}
	return outcome{code: code, limited: limited, resets: resets}, nil
}

// harness builds the named harness, its environment and the proxy's credentials.
// Claude Code's token goes in the VM, which can send it only to Anthropic through
// the proxy (adr-0043); OpenRouter's key stays on the host and the proxy adds it.
func (r *Runner) harness(
	name string,
) (interactive, map[string]string, vm.EgressConfig, error) {
	egress := vm.EgressConfig{
		Server: "", MCPToken: nil, OpenRouterKey: nil, PostHogKey: nil, MCPRejected: nil,
		Transport: nil, Anthropic: "", OpenRouter: "", PostHog: "", GoProxy: "", NPM: "",
		RubyGems: "", Log: nil,
	}
	switch name {
	case Claude:
		token, err := r.Secrets.Get(SecretAnthropic)
		if err != nil {
			return nil, nil, egress, secretErr(SecretAnthropic, err)
		}
		env := map[string]string{"CLAUDE_CODE_OAUTH_TOKEN": token}
		return vm.ClaudeCode{}, env, egress, nil
	case OpenCode:
		if r.Config.OpenCode.Model == "" {
			return nil, nil, egress, errors.New("set opencode.model in the wardcase config")
		}
		if _, err := r.Secrets.Get(SecretOpenRouter); err != nil {
			return nil, nil, egress, secretErr(SecretOpenRouter, err)
		}
		egress.OpenRouterKey = func() (string, error) {
			return r.Secrets.Get(SecretOpenRouter)
		}
		return vm.OpenCode{Model: r.Config.OpenCode.Model}, map[string]string{}, egress, nil
	}
	return nil, nil, egress, fmt.Errorf("unknown harness %q", name)
}

func secretErr(name string, err error) error {
	return fmt.Errorf("the %s secret: %w (wardcase secret set %s)", name, err, name)
}

func maps(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

func randomID() string {
	b := make([]byte, 6) //nolint:mnd // 12 hex characters
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// limitWriter passes a headless harness's output through and watches each line for
// its usage limit.
type limitWriter struct {
	h       vm.Harness
	out     io.Writer
	buf     []byte
	limited bool
	resets  time.Time
}

func (w *limitWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.line(w.buf[:i])
		w.buf = w.buf[i+1:]
	}
	if _, err := w.out.Write(p); err != nil {
		return 0, fmt.Errorf("write output: %w", err)
	}
	return len(p), nil
}

func (w *limitWriter) line(l []byte) {
	if hit, at := w.h.LimitFrom(l); hit {
		w.limited, w.resets = true, at
	}
}

func (w *limitWriter) flush() {
	if len(w.buf) > 0 {
		w.line(w.buf)
		w.buf = nil
	}
}
