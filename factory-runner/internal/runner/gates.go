package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// gatesFile lists a repo's gates, one `prefix | command` per line.
	gatesFile = ".factory/gates"
	gateLimit = 20 * time.Minute
	gateTail  = 3000
	bashCmd   = "bash"
)

type hostGate struct{ prefix, command string }

// parseGates reads the gates file; blank and # lines are skipped.
func parseGates(text string) []hostGate {
	var gates []hostGate
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		prefix, command, ok := strings.Cut(line, "|")
		if line == "" || strings.HasPrefix(line, "#") || !ok ||
			strings.TrimSpace(prefix) == "" {
			continue
		}
		gates = append(gates, hostGate{prefix: strings.TrimSpace(prefix),
			command: strings.TrimSpace(command)})
	}
	return gates
}

// gatesFor keeps the gates whose prefix matches a changed path; "*" always does.
func gatesFor(gates []hostGate, changed []string) []hostGate {
	var out []hostGate
	for _, g := range gates {
		match := g.prefix == "*"
		for _, c := range changed {
			match = match || strings.HasPrefix(c, g.prefix)
		}
		if match {
			out = append(out, g)
		}
	}
	return out
}

// runGates runs the repo's matching gates in the job VM, returning the first
// failure's feedback for the agent and its one-line reason for the task, or ""
// when all pass or none are defined.
func (l *jobLoop) runGates() (string, string) {
	began := l.now()
	defer func() { l.times.gates += l.now().Sub(began) }()
	// The base branch's list, so the agent cannot edit its own gates away.
	raw, err := l.r.Git.git(l.ctx, l.dirs.clone, "cat-file", "blob",
		"origin/main:"+gatesFile)
	if err != nil {
		return "", ""
	}
	gates := parseGates(string(raw))
	if len(gates) == 0 {
		return "", ""
	}
	if c := l.syncBase(); c != "" {
		return c, short(c)
	}
	var diff bytes.Buffer
	code, err := l.session.Exec(l.ctx, []string{bashCmd, "-c",
		"git diff --name-only origin/main...HEAD || git ls-files"}, "", &diff)
	changed := strings.Fields(diff.String())
	if err != nil || code != 0 {
		changed = nil
	}
	for _, g := range gatesFor(gates, changed) {
		ctx, cancel := context.WithTimeout(l.ctx, gateLimit)
		var out bytes.Buffer
		// Not a login shell: /etc/profile would reset the image's PATH (go).
		gateCode, runErr := l.session.Exec(ctx, []string{bashCmd, "-c", g.command}, "", &out)
		cancel()
		if runErr == nil && gateCode == 0 {
			continue
		}
		if l.ended() != "" {
			return "", ""
		}
		tail := redact(gateExcerpt(out.String(), gateTail), l.events.redactions)
		return fmt.Sprintf("The gate `%s` ran before pushing and failed:"+
			"\n\n%s\n\nFix it, run it yourself, commit, and write the outcome again.",
			g.command, tail), gateWhy(g.command, tail)
	}
	return "", ""
}

// gateWhy is a failed gate's reason on one line: its command and the end of its
// output, where the error is, short enough that "gate failed: " and short()
// keep that end.
func gateWhy(command, excerpt string) string {
	s := strings.Join(strings.Fields(excerpt), " ")
	room := maxWhy - len("gate failed: `") - len(command) - len("`: ...")
	if len(s) > room {
		s = "..." + strings.ToValidUTF8(s[len(s)-max(room, 0):], "")
	}
	return "`" + command + "`: " + s
}

const (
	excerptContext = 2
	excerptTail    = 15
)

// gateNoise reports passing-package lines that bury the failure.
func gateNoise(line string) bool {
	return strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "ok\t") ||
		strings.Contains(line, "(cached)") || strings.Contains(line, "[no test files]")
}

func gateFailure(line string) bool {
	if strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "FAIL ") {
		return true
	}
	for _, m := range []string{"--- FAIL", "panic:", "Error", "error:", "make: ***"} {
		if strings.Contains(line, m) {
			return true
		}
	}
	return false
}

// gateExcerpt keeps the failure lines of a gate's output with a little context,
// then the output tail if it fits, within limit bytes. Over the limit, the
// earliest failures go first.
func gateExcerpt(output string, limit int) string {
	var lines []string
	for _, l := range strings.Split(output, "\n") {
		if !gateNoise(l) {
			lines = append(lines, l)
		}
	}
	keep := make([]bool, len(lines))
	mark := func(from, to int) {
		for i := max(from, 0); i <= to && i < len(lines); i++ {
			keep[i] = true
		}
	}
	for i, l := range lines {
		if gateFailure(l) {
			mark(i-excerptContext, i+excerptContext)
		}
	}
	text := joinKept(lines, keep)
	if text == "" {
		text = strings.Join(lines, "\n")
	} else if len(text) <= limit {
		withTail := append([]bool(nil), keep...)
		for i := max(len(lines)-excerptTail, 0); i < len(lines); i++ {
			withTail[i] = true
		}
		if t := joinKept(lines, withTail); len(t) <= limit {
			text = t
		}
	}
	if len(text) > limit {
		text = text[len(text)-limit:]
	}
	return strings.ToValidUTF8(text, "")
}

func joinKept(lines []string, keep []bool) string {
	var b strings.Builder
	gap := false
	for i, l := range lines {
		if !keep[i] {
			gap = b.Len() > 0
			continue
		}
		if gap {
			b.WriteString("...\n")
			gap = false
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
