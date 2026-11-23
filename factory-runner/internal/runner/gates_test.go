package runner_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const gatesText = "# c\napi/ | make api\nweb/ | make web\n\n* | make docs\nbad line\n"

func TestGatesAreScopedByChangedPaths(t *testing.T) {
	assert.Len(t, runner.ParseGatesForTest(gatesText), 3)
	assert.Equal(t, []string{"make api", "make docs"},
		runner.GatesForTest(gatesText, []string{"api/a.go"}))
	assert.Equal(t, []string{"make docs"}, runner.GatesForTest(gatesText, nil))
}

func TestAFailingHostGateGoesBackToTheAgentBeforeAnyPush(t *testing.T) {
	agentN, gateN := 0, 0
	p := newPublishing(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		if len(spec.Command) > 1 && spec.Command[0] == "bash" {
			if strings.HasPrefix(spec.Command[2], "git diff") {
				_, _ = fmt.Fprintln(spec.Log, "api/a.go")
				return 0, nil
			}
			gateN++
			if gateN == 1 {
				_, _ = fmt.Fprintln(spec.Log, strings.Repeat("x", 5000)+"lint boom")
				return 1, nil
			}
			return 0, nil
		}
		agentN++
		return agentRuns("Try"+passAll, true)(agentN, spec), nil
	})

	p.runner.Git.Run = func(
		ctx context.Context, dir string, args ...string,
	) ([]byte, error) {
		if len(args) > 0 && args[0] == "cat-file" {
			return []byte("api/ | make lint\n"), nil
		}
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		return cmd.CombinedOutput()
	}

	_, err := p.runner.Next(context.Background())
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(p.sandbox.specs), 6)
	for _, sp := range p.sandbox.specs {
		if len(sp.Command) > 2 && sp.Command[2] == "make lint" {
			assert.Equal(t, []string{"bash", "-c", "make lint"}, sp.Command,
				"a login shell would reset PATH and lose go")
		}
	}
	assert.Contains(t, p.sandbox.specs[3].Stdin, "lint boom")
	assert.Contains(t, p.sandbox.specs[3].Stdin, "make lint")
	assert.Less(t, len(p.sandbox.specs[3].Stdin), 3400, "output is cut to its tail")
	p.api.mu.Lock()
	comments := strings.Join(p.api.comments, "\n")
	p.api.mu.Unlock()
	assert.Contains(t, comments, "gate failed: `make lint`: ...")
	assert.Contains(t, comments, "lint boom",
		"the task hears the error, not the output's head")
}

func TestGateWhyKeepsTheEndOfTheOutput(t *testing.T) {
	excerpt := "du: cannot access gc-socket: Operation not supported\n" +
		strings.Repeat("noise\n", 200) + "make: *** [lint/docs] Error 1"

	why := runner.GateWhyForTest("make lint/docs", excerpt)

	assert.True(t, strings.HasPrefix(why, "`make lint/docs`: ..."), why)
	assert.True(t, strings.HasSuffix(why, "make: *** [lint/docs] Error 1"), why)
	assert.NotContains(t, why, "gc-socket")
	assert.Equal(t, "gate failed: "+why, runner.ShortForTest("gate failed: "+why),
		"short() keeps it whole")
	assert.Equal(t, "`true`: a b", runner.GateWhyForTest("true", "a\n  b\n"))
}

func TestGateExcerptKeepsTheFailingPackage(t *testing.T) {
	var b strings.Builder
	b.WriteString("go test ./...\n")
	for i := range 120 {
		fmt.Fprintf(&b, "ok  \ttools.xdoubleu.com/internal/pkg%d\t(cached)\n", i)
		fmt.Fprintf(&b, "?   \ttools.xdoubleu.com/internal/gen%d\t[no test files]\n", i)
	}
	b.WriteString("--- FAIL: TestParse (0.00s)\n" +
		"    parse_test.go:42: \n" +
		"        \tError Trace:\tparse_test.go:42\n" +
		"        \tError:      \tNot equal: \n" +
		"        \t            \texpected: 3\n" +
		"        \t            \tactual  : 4\n" +
		"FAIL\n" +
		"FAIL\ttools.xdoubleu.com/internal/connecttools\t0.012s\n")
	for i := range 120 {
		fmt.Fprintf(&b, "ok  \ttools.xdoubleu.com/internal/late%d\t0.1s\n", i)
	}
	b.WriteString("FAIL\nmake: *** [Makefile:9: test] Error 1\n")

	got := runner.GateExcerptForTest(b.String(), 3000)

	assert.LessOrEqual(t, len(got), 3000)
	assert.Contains(t, got, "--- FAIL: TestParse")
	assert.Contains(t, got, "expected: 3")
	assert.Contains(t, got, "FAIL\ttools.xdoubleu.com/internal/connecttools")
	assert.Contains(t, got, "make: ***")
	assert.NotContains(t, got, "(cached)")
	assert.NotContains(t, got, "[no test files]")
	assert.NotContains(t, got, "pkg5\t")
}

func TestGateExcerptTruncatesToTheLimit(t *testing.T) {
	got := runner.GateExcerptForTest(strings.Repeat("panic: boom\n", 100), 100)
	assert.LessOrEqual(t, len(got), 100)
	assert.Contains(t, got, "panic: boom")
}
