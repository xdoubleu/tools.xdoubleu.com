package runner_test

import (
	"testing"
	"time"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestPhaseTimes(t *testing.T) {
	t0 := time.Unix(0, 0)
	var p runner.PhaseTimes
	p.AddWatch(t0, t0.Add(2*time.Minute), t0.Add(3*time.Minute))
	p.AddWatch(t0, time.Time{}, t0.Add(time.Minute))
	p.SetAgent(5 * time.Second)
	want := "Phase times: agent 5s, gates 0s, push/PR 0s, CI 3m0s, " +
		"merge wait 1m0s (total 10m0s)"
	if got := p.Summary(10 * time.Minute); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
