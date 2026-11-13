package runner_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

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

func sandboxWith(t *testing.T, exec *scriptedExec) *runner.ContainerSandbox {
	t.Helper()
	return &runner.ContainerSandbox{
		Image:      "factory-job:test",
		Exec:       exec,
		Proxy:      nil,
		GoModCache: "",
		NPMCache:   "",
		CPUs:       "",
		Memory:     "",
		ProxyPort:  0,
		TempDir:    t.TempDir(),
	}
}

func spec(t *testing.T, log io.Writer) runner.SandboxSpec {
	t.Helper()
	return runner.SandboxSpec{
		CloneDir:      "/data/jobs/j1/clone",
		OutcomeDir:    "/data/jobs/j1/outcome",
		SessionDir:    "/data/sessions/t1",
		Env:           map[string]string{"FACTORY_JOB": "0a1b2c3d-4e5f", "SECRET": "s3cret"},
		SetupCommands: "npm ci",
		Command:       []string{"claude", "-p"},
		Proxy:         nil,
		Stdin:         "do the thing",
		Log:           log,
	}
}

func routesAnswer(argv []string) (int, string, error) {
	if argv[0] == "netstat" {
		return 0, hostRoutes, nil
	}
	return 0, "", nil
}

func TestTheSandboxStartsAPlaceholderVMOnItsOwnNetworkThenRunsTheJobInIt(t *testing.T) {
	var envFile, envBody string
	exec := newExecScript(routesAnswer)
	exec.onExec = func(argv []string, stdin string) {
		assert.Equal(t, "do the thing", stdin)
		for i, a := range argv {
			if a == "--env-file" {
				envFile = argv[i+1]
			}
		}
		body, err := os.ReadFile(envFile)
		require.NoError(t, err)
		envBody = string(body)
		info, err := os.Stat(envFile)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	var gateway string
	box := sandboxWith(t, exec)
	box.Proxy = &fakeProxy{
		gateway: &gateway, err: nil, port: nil,
		onStop: func() {
			exec.mu.Lock()
			defer exec.mu.Unlock()
			exec.calls = append(exec.calls, []string{"proxy", "stop"})
		},
	}

	code, err := box.Run(context.Background(), spec(t, &bytes.Buffer{}))
	require.NoError(t, err)
	assert.Zero(t, code)

	assert.Equal(t, "10.231.7.1", gateway)
	assert.Equal(t, []string{
		"kill", "delete", "network delete", // whatever a crash left behind
		"netstat -rn", "network create", "run", "exec",
		"proxy stop", // before the VM goes
		"kill", "delete", "network delete",
	}, exec.steps())

	create := exec.find("network")[1]
	assert.Contains(t, strings.Join(create, " "), "--internal --subnet 10.231.7.0/24")

	run := strings.Join(exec.find("run")[0], " ")
	for _, want := range []string{
		"--read-only", "--tmpfs /tmp", "--cap-drop ALL", "--user 1000:1000",
		"--network factory-job-0a1b2c3d4e5f",
		"type=bind,source=/data/jobs/j1/clone,target=/work",
		"type=bind,source=/data/jobs/j1/outcome,target=/outcome",
		"type=bind,source=/data/sessions/t1,target=/session",
		"--entrypoint sleep factory-job:test infinity",
	} {
		assert.Contains(t, run, want)
	}
	assert.NotContains(t, run, "readonly", "no cache mounts unless configured")

	assert.Contains(t, envBody, "FACTORY_JOB=0a1b2c3d-4e5f\n")
	assert.Contains(t, envBody, "GOPROXY=file:///opt/gomodcache/cache/download,http://")
	assert.Contains(t, envBody, "/goproxy/\n")
	assert.Contains(t, envBody, "/npm/\n")
	assert.Contains(t, envBody, "FACTORY_GATEWAY=10.231.7.1\n")
	assert.NoFileExists(t, envFile, "the env file is removed afterwards")

	// The secret is in the file, never in argv.
	for _, c := range exec.calls {
		assert.NotContains(t, strings.Join(c, " "), "s3cret")
	}
	execLine := strings.Join(exec.find("exec")[0], " ")
	assert.Contains(t, execLine, "--workdir /work factory-job-0a1b2c3d4e5f sh -c")
	assert.Contains(t, execLine, "npm ci")
	assert.True(t, strings.HasSuffix(execLine, "exec \"$@\" sh claude -p"), execLine)
}

func TestTheSandboxMountsTheRegistryCachesReadOnlyWhenConfigured(t *testing.T) {
	exec := newExecScript(routesAnswer)
	box := sandboxWith(t, exec)
	box.GoModCache = "/cache/go"
	box.NPMCache = "/cache/npm"
	box.CPUs = "2"
	box.Memory = "4G"

	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)

	run := strings.Join(exec.find("run")[0], " ")
	assert.Contains(t, run, "type=bind,source=/cache/go,target=/gomodcache,readonly")
	assert.Contains(t, run, "type=bind,source=/cache/npm,target=/npmcache,readonly")
	assert.Contains(t, run, "--cpus 2 --memory 4G")
}

func TestTheJobsExitCodeIsReturnedAndTheVMStillCleanedUp(t *testing.T) {
	exec := newExecScript(func(argv []string) (int, string, error) {
		if len(argv) > 1 && argv[1] == "exec" {
			return 3, "", nil
		}
		return routesAnswer(argv)
	})
	code, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	assert.Equal(t, 3, code)
	last := exec.calls[len(exec.calls)-1]
	assert.Equal(t, []string{"container", "network", "delete", "factory-job-0a1b2c3d4e5f"},
		last)
}

func TestANetworkThatCannotBeCreatedOnOneSubnetTriesTheNext(t *testing.T) {
	exec := newExecScript(nil)
	exec.answer = func(argv []string) (int, string, error) {
		if argv[0] == "netstat" {
			return 0, hostRoutes, nil
		}
		if len(argv) > 3 && argv[2] == "create" && strings.Contains(strings.Join(argv, " "),
			"10.231.7.0/24") {
			return 1, "subnet in use", nil
		}
		return 0, "", nil
	}
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	creates := exec.find("network")
	var subnets []string
	for _, c := range creates {
		if c[2] == "create" {
			subnets = append(subnets, c[len(c)-2])
		}
	}
	assert.Equal(t, []string{"10.231.7.0/24", "10.231.8.0/24"}, subnets)
}

func TestNoFreeSubnetFailsBeforeAnyVMStarts(t *testing.T) {
	exec := newExecScript(func(argv []string) (int, string, error) {
		if argv[0] == "netstat" {
			return 0, "10/8   10.0.0.1   UGSc   en0\n172.16/12 x UGSc en1\n" +
				"192.168/16 x UGSc en2\n", nil
		}
		return 0, "", nil
	})
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.ErrorContains(t, err, "no free subnet")
	assert.Empty(t, exec.find("run"))
}

func TestEverySubnetRefusedReportsWhyAndStartsNoVM(t *testing.T) {
	exec := newExecScript(func(argv []string) (int, string, error) {
		if argv[0] == "netstat" {
			return 0, hostRoutes, nil
		}
		if len(argv) > 2 && argv[2] == "create" {
			return 1, "no vmnet", nil
		}
		return 0, "", nil
	})
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.ErrorContains(t, err, "no vmnet")
	assert.Empty(t, exec.find("run"))
}

func TestARouteListThatCannotBeReadFailsTheJob(t *testing.T) {
	exec := newExecScript(func(argv []string) (int, string, error) {
		if argv[0] == "netstat" {
			return -1, "", errors.New("netstat: not found")
		}
		return 0, "", nil
	})
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.ErrorContains(t, err, "reading host routes")
}

func TestAVMThatFailsToStartFailsTheJobAndIsCleanedUp(t *testing.T) {
	exec := newExecScript(func(argv []string) (int, string, error) {
		if len(argv) > 1 && argv[1] == "run" {
			return 1, "image not found", nil
		}
		return routesAnswer(argv)
	})
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.ErrorContains(t, err, "image not found")
	assert.Empty(t, exec.find("exec"))
	assert.Equal(t, "network", exec.calls[len(exec.calls)-1][1])
}

func TestAProxyThatCannotStartStopsTheJobBeforeTheAgentRuns(t *testing.T) {
	exec := newExecScript(routesAnswer)
	box := sandboxWith(t, exec)
	box.Proxy = &fakeProxy{
		gateway: new(string), err: errors.New("address in use"), port: nil,
		onStop: func() {},
	}
	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.ErrorContains(t, err, "address in use")
	assert.Empty(t, exec.find("exec"))
	assert.NotEmpty(t, exec.find("kill"))
}

func TestAnEnvironmentValueWithALineBreakIsRefused(t *testing.T) {
	exec := newExecScript(routesAnswer)
	s := spec(t, io.Discard)
	s.Env["BAD"] = "a\nPATH=/evil"
	_, err := sandboxWith(t, exec).Run(context.Background(), s)
	require.ErrorContains(t, err, "line break")
	assert.Empty(t, exec.find("run"))
}

func TestCheckAcceptsOnlyTheTestedContainerRelease(t *testing.T) {
	for name, tc := range map[string]struct {
		code    int
		out     string
		err     error
		wantErr string
	}{
		"supported":  {0, "container CLI version 1.6.1 (build: release)", nil, ""},
		"newer":      {0, "container CLI version 1.7.0", nil, "1.6.x is required"},
		"older":      {0, "container CLI version 1.5.9", nil, "1.6.x is required"},
		"missing":    {-1, "", errors.New("container: not found"), "not usable"},
		"bad status": {2, "boom", nil, "not usable"},
	} {
		exec := newExecScript(func([]string) (int, string, error) {
			return tc.code, tc.out, tc.err
		})
		err := sandboxWith(t, exec).Check(context.Background())
		if tc.wantErr == "" {
			require.NoError(t, err, name)
		} else {
			require.ErrorContains(t, err, tc.wantErr, name)
		}
	}
}

func TestSubnetsAvoidEveryHostRouteThatOverlaps(t *testing.T) {
	routes := hostRoutes +
		"10.231.7/24       link#20            UC            bridge100\n" +
		"10.231.8.0/22     link#21            UC            utun3\n" +
		"10.231.20.5       link#21            UHW           utun3\n" +
		"192.168           link#2             UC            en1\n"
	got := runner.FreeSubnets(routes, 3)
	assert.Equal(t, []string{"10.231.12.0/24", "10.231.13.0/24", "10.231.14.0/24"}, got)
}

func TestARouteTableWithNoOverlapStartsAtTheFirstSubnet(t *testing.T) {
	assert.Equal(t, []string{"10.231.7.0/24"}, runner.FreeSubnets(hostRoutes, 1))
}

func TestAVPNHoldingTenSlashEightMovesTheJobNetworkToAnotherRange(t *testing.T) {
	exec := newExecScript(func(argv []string) (int, string, error) {
		if argv[0] == "netstat" {
			return 0, "10/8   10.0.0.1   UGSc   utun3\n", nil
		}
		return 0, "", nil
	})
	var gateway string
	box := sandboxWith(t, exec)
	box.Proxy = &fakeProxy{
		gateway: &gateway, err: nil, port: nil, onStop: func() {},
	}
	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	assert.Equal(t, "172.29.7.1", gateway)
}

func TestSetupCommandsReadNoStdinSoThePromptReachesTheAgent(t *testing.T) {
	exec := newExecScript(routesAnswer)
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)
	script := exec.find("exec")[0][len(exec.find("exec")[0])-4]
	assert.Contains(t, script, "(\nnpm ci\n) </dev/null\nexec \"$@\"")
}

func TestASpecTheVMCannotCarryFaithfullyIsRefusedBeforeAnythingStarts(t *testing.T) {
	for name, mutate := range map[string]func(*runner.SandboxSpec){
		"bad env name":  func(s *runner.SandboxSpec) { s.Env["A=B"] = "x" },
		"empty env":     func(s *runner.SandboxSpec) { s.Env[""] = "x" },
		"comma clone":   func(s *runner.SandboxSpec) { s.CloneDir = "/data/a,b/clone" },
		"comma outcome": func(s *runner.SandboxSpec) { s.OutcomeDir = "/data/a,b" },
		"newline home":  func(s *runner.SandboxSpec) { s.SessionDir = "/data/a\nb" },
	} {
		exec := newExecScript(routesAnswer)
		s := spec(t, io.Discard)
		mutate(&s)
		_, err := sandboxWith(t, exec).Run(context.Background(), s)
		require.Error(t, err, name)
		assert.Empty(t, exec.calls, name)
	}

	exec := newExecScript(routesAnswer)
	box := sandboxWith(t, exec)
	box.GoModCache = "/cache/a,readonly=false"
	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.ErrorContains(t, err, "cannot be mounted")
	assert.Empty(t, exec.find("exec"))
}
