package runner

import (
	"errors"
	"fmt"
	"strings"
)

// errChecks is why a finished job's quality gates don't allow a pull request.
var errChecks = errors.New("checks")

// ciCovered is the note that marks a gate the sandbox can't run but CI does, as in
// `build=not run (CI checks it)`.
const ciCovered = "ci checks it"

// checksFailure is nil when the outcome has a `Checks:` line and every gate on
// it passed or was not run because CI checks it. The line looks like
// `Checks: lint=pass, test=pass, docs=pass`; a gate is pass, fail or "not run",
// and a "not run" followed by "(CI checks it)" is left to CI.
func checksFailure(outcome string) error {
	line, ok := findLine(outcome, "Checks:")
	if !ok {
		return fmt.Errorf("%w: the outcome has no `Checks:` line", errChecks)
	}
	var bad []string
	n := 0
	for _, part := range splitGates(line) {
		name, status, found := strings.Cut(part.text, "=")
		name = strings.TrimSpace(name)
		status = strings.ToLower(strings.Join(strings.Fields(status), " "))
		if !found || name == "" {
			return fmt.Errorf("%w: malformed `Checks:` line %q", errChecks, line)
		}
		n++
		switch {
		case status == "pass":
		case status != "fail" && strings.Contains(part.note, ciCovered):
		case status == "fail":
			bad = append(bad, name+" fail")
		default:
			bad = append(bad, name+" not run")
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

// gate is one entry of a `Checks:` line: its text outside parentheses, and the
// lowercased text of its parenthesized notes.
type gate struct{ text, note string }

// splitGates splits a `Checks:` line at top-level commas and semicolons, so a note
// such as "test=fail (web not run; see above)" stays with its gate.
func splitGates(line string) []gate {
	var (
		gates []gate
		text  strings.Builder
		note  strings.Builder
		depth int
	)
	flush := func() {
		if strings.TrimSpace(text.String()) != "" || note.Len() > 0 {
			gates = append(gates, gate{
				text: strings.TrimSpace(text.String()),
				note: strings.ToLower(strings.Join(strings.Fields(note.String()), " ")),
			})
		}
		text.Reset()
		note.Reset()
	}
	for _, r := range line {
		switch {
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
			note.WriteRune(' ')
		case depth > 0:
			note.WriteRune(r)
		case r == ',' || r == ';':
			flush()
		default:
			text.WriteRune(r)
		}
	}
	flush()
	return gates
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
