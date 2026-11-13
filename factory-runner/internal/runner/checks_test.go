package runner_test

import (
	"strings"
	"testing"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestChecksFailure(t *testing.T) {
	cases := map[string]string{
		"Title\n\nChecks: lint=pass, test=pass, docs=pass\n":           "",
		"Checks: lint=pass; test=fail":                                 "test fail",
		"Checks: lint=pass, test=not run":                              "test not run",
		"Checks: lint=PASS, test=maybe":                                "test not run",
		"Title\nno gates":                                              "no `Checks:` line",
		"Checks: lint=pass (go), test=pass (a, b; c)":                  "",
		"Checks: lint=pass (go; web not run), test=fail (x)":           "test fail",
		"Checks: lint=pass, mac=not run (CI checks it)":                "",
		"Checks: lint=pass, mac=NOT RUN (the macOS job, CI checks it)": "",
		"Checks: lint=pass, mac=not run (CI checks it), test=not run":  "test not run",
		"Checks: mac=fail (CI checks it)":                              "mac fail",
		"Checks: mac=not run (no time)":                                "mac not run",
		"Checks: lint pass":                                            "malformed",
		"Checks:   ":                                                   "lists no gate",
	}
	for in, want := range cases {
		err := runner.ChecksFailureForTest(in)
		switch {
		case want == "" && err != nil:
			t.Errorf("%q: unexpected %v", in, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%q: got %v, want %q", in, err, want)
		}
	}
}
