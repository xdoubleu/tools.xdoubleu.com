package runner_test

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// scriptedExec mirrors wardcase/vm's test helper for the runner's own tests.

// scriptedExec records every host command and answers from a handler.
type scriptedExec struct {
	mu     sync.Mutex
	calls  [][]string
	answer func(argv []string) (int, string, error)
	// onExec sees a `container exec` as it runs, while the env file still exists.
	onExec func(argv []string, stdin string)
}

func (e *scriptedExec) Exec(
	_ context.Context,
	argv []string,
	stdin io.Reader,
	out io.Writer,
) (int, error) {
	e.mu.Lock()
	e.calls = append(e.calls, argv)
	e.mu.Unlock()
	if len(argv) > 1 && argv[1] == "exec" && e.onExec != nil {
		var in []byte
		if stdin != nil {
			in, _ = io.ReadAll(stdin)
		}
		e.onExec(argv, string(in))
	}
	code, text, err := 0, "", error(nil)
	if e.answer != nil {
		code, text, err = e.answer(argv)
	}
	_, _ = io.WriteString(out, text)
	return code, err
}

func (e *scriptedExec) find(verb string) [][]string {
	var found [][]string
	for _, c := range e.calls {
		if len(c) > 1 && strings.Join(c[:2], " ") == "container "+verb {
			found = append(found, c)
		}
	}
	return found
}

func newExecScript(answer func(argv []string) (int, string, error)) *scriptedExec {
	return &scriptedExec{
		mu:     sync.Mutex{},
		calls:  nil,
		answer: answer,
		onExec: nil,
	}
}

// steps names each recorded command by what it does, in order.
func (e *scriptedExec) steps() []string {
	var out []string
	for _, c := range e.calls {
		switch {
		case c[0] != "container":
			out = append(out, c[0]+" "+c[1])
		case c[1] == "network":
			out = append(out, "network "+c[2])
		default:
			out = append(out, c[1])
		}
	}
	return out
}

const hostRoutes = `Routing tables

Internet:
Destination        Gateway            Flags               Netif Expire
default            192.168.128.1      UGScg                 en0
127                127.0.0.1          UCS                   lo0
192.168.128/23     link#11            UCS                   en0      !
`

func routesAnswer(argv []string) (int, string, error) {
	if argv[0] == "netstat" {
		return 0, hostRoutes, nil
	}
	return 0, "", nil
}

type fakeProxy struct {
	gateway *string
	port    *int
	err     error
	onStop  func()
}

func sandboxWith(t *testing.T, exec *scriptedExec) *runner.ContainerSandbox {
	t.Helper()
	return &runner.ContainerSandbox{
		Image:      "factory-job:test",
		Exec:       exec,
		Proxy:      nil,
		GoModCache: "",
		NPMCache:   "",
		NixStore:   "", NixStoreMax: 0,
		CPUs:      "",
		Memory:    "",
		ProxyPort: 0,
		TempDir:   t.TempDir(),
	}
}

func spec(t *testing.T, log io.Writer) runner.SandboxSpec {
	t.Helper()
	return runner.SandboxSpec{
		CloneDir:    "/data/jobs/j1/clone",
		OutcomeDir:  "/data/jobs/j1/outcome",
		SessionDir:  "/data/sessions/t1",
		Env:         map[string]string{"FACTORY_JOB": "0a1b2c3d-4e5f", "SECRET": "s3cret"},
		Command:     []string{"claude", "-p"},
		Proxy:       nil,
		Stdin:       "do the thing",
		Log:         log,
		Mounts:      nil,
		SkipInstall: false,
	}
}
func (f *fakeProxy) Listen(gateway string, port int) (func(), string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	*f.gateway = gateway
	if f.port != nil {
		*f.port = port
	}
	return f.onStop, net.JoinHostPort(gateway, strconv.Itoa(port)), nil
}
