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
// failure's feedback for the agent, or "" when all pass or none are defined.
func (l *jobLoop) runGates() string {
	began := l.now()
	defer func() { l.times.gates += l.now().Sub(began) }()
	// The base branch's list, so the agent cannot edit its own gates away.
	raw, err := l.r.Git.git(l.ctx, l.dirs.clone, "cat-file", "blob",
		"origin/main:"+gatesFile)
	if err != nil {
		return ""
	}
	gates := parseGates(string(raw))
	if len(gates) == 0 {
		return ""
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
		gateCode, runErr := l.session.Exec(ctx, []string{bashCmd, "-lc", g.command}, "", &out)
		cancel()
		if runErr == nil && gateCode == 0 {
			continue
		}
		if l.ended() != "" {
			return ""
		}
		tail := strings.ToValidUTF8(out.String(), "")
		if len(tail) > gateTail {
			tail = tail[len(tail)-gateTail:]
		}
		return fmt.Sprintf("The gate `%s` before pushing and it failed:"+
			"\n\n%s\n\nFix it, run it yourself, commit, and write the outcome again.",
			g.command, redact(tail, l.events.redactions))
	}
	return ""
}
