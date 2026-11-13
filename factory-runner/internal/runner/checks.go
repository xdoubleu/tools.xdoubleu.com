package runner

import (
	"errors"
	"fmt"
	"strings"
)

// errChecks is why a finished job's quality gates don't allow a pull request.
var errChecks = errors.New("checks")

// checksFailure is nil when the outcome has a `Checks:` line and every gate on
// it passed. The line looks like `Checks: lint=pass, test=pass, docs=pass`; a
// gate is pass, fail or "not run".
func checksFailure(outcome string) error {
	line, ok := findLine(outcome, "Checks:")
	if !ok {
		return fmt.Errorf("%w: the outcome has no `Checks:` line", errChecks)
	}
	var bad []string
	n := 0
	for part := range strings.FieldsFuncSeq(dropNotes(line), func(r rune) bool {
		return r == ',' || r == ';'
	}) {
		name, status, found := strings.Cut(strings.TrimSpace(part), "=")
		name = strings.TrimSpace(name)
		status = strings.ToLower(strings.Join(strings.Fields(status), " "))
		if !found || name == "" {
			return fmt.Errorf("%w: malformed `Checks:` line %q", errChecks, line)
		}
		n++
		if status != "pass" {
			if status != "fail" {
				status = "not run"
			}
			bad = append(bad, name+" "+status)
		}
	}
	switch {
	case n == 0:
		return fmt.Errorf("%w: the `Checks:` line lists no gate", errChecks)
	case len(bad) > 0:
		return fmt.Errorf("%w: not every check passed (%s)",
			errChecks, strings.Join(bad, ", "))
	}
	return nil
}

// dropNotes removes parenthesized notes, such as "test=fail (web not run; see
// above)", whose separators would otherwise split a gate.
func dropNotes(line string) string {
	var b strings.Builder
	depth := 0
	for _, r := range line {
		switch {
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// findLine returns the text after the last line starting with prefix.
func findLine(text, prefix string) (string, bool) {
	var (
		found string
		ok    bool
	)
	for l := range strings.SplitSeq(text, "\n") {
		if rest, has := strings.CutPrefix(strings.TrimSpace(l), prefix); has {
			found, ok = strings.TrimSpace(rest), true
		}
	}
	return found, ok
}
