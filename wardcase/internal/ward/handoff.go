package ward

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tools.xdoubleu.com/wardcase/vm"
)

const (
	// HandoffFile is the handoff's name in the next harness's session directory.
	HandoffFile = "wardcase-handoff.md"

	maxMessage    = 2000
	maxTranscript = 30000
	// transcriptTail is how much of a transcript is read for its last reply.
	transcriptTail = 256 << 10
	maxLineBytes   = 16 << 20
	lineBufBytes   = 64 << 10
)

// entry is one line of a Claude Code transcript (`projects/<dir>/<session>.jsonl`).
type entry struct {
	Type             string `json:"type"`
	IsMeta           bool   `json:"isMeta"`
	IsCompactSummary bool   `json:"isCompactSummary"`
	Message          struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// text is an entry's own words: a plain string, or its text blocks without tool
// calls and their results.
func (e entry) text() string {
	var s string
	if json.Unmarshal(e.Message.Content, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(e.Message.Content, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// latestTranscript is the most recently written Claude Code transcript under a
// session directory (CLAUDE_CONFIG_DIR), or "".
func latestTranscript(sessionDir string) string {
	files, _ := filepath.Glob(filepath.Join(sessionDir, "projects", "*", "*.jsonl"))
	var newest string
	var at time.Time
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil && info.ModTime().After(at) {
			newest, at = f, info.ModTime()
		}
	}
	return newest
}

func readEntries(path string) []entry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var out []entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, lineBufBytes), maxLineBytes)
	for sc.Scan() {
		var e entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Type == "user" || e.Type == "assistant" {
			out = append(out, e)
		}
	}
	return out
}

// ClaudeLimited reports whether the last reply of the session's latest transcript
// is Claude Code's usage-limit message, and when the limit lifts if it says.
func ClaudeLimited(sessionDir string) (bool, time.Time) {
	path := latestTranscript(sessionDir)
	if path == "" {
		return false, time.Time{}
	}
	entries := readEntries(path)
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Type == "assistant" {
			return vm.LimitInText(entries[i].text())
		}
	}
	return false, time.Time{}
}

// compactTranscript is the conversation of the session's latest Claude Code
// transcript: the user's messages and the agent's replies, without tool calls or
// their output, from Claude's own last compact summary on, each one capped and the
// whole kept to its most recent part.
func compactTranscript(sessionDir string) string {
	path := latestTranscript(sessionDir)
	if path == "" {
		return ""
	}
	entries := readEntries(path)
	start := 0
	for i, e := range entries {
		if e.IsCompactSummary {
			start = i
		}
	}
	var b strings.Builder
	for _, e := range entries[start:] {
		text := strings.TrimSpace(e.text())
		if text == "" || e.IsMeta {
			continue
		}
		who := "Agent"
		switch {
		case e.IsCompactSummary:
			who = "Summary of the earlier conversation"
		case e.Type == "user":
			who = "User"
		}
		fmt.Fprintf(&b, "**%s:** %s\n\n", who, capText(text, maxMessage))
	}
	out := b.String()
	if len(out) > maxTranscript {
		out = "…(earlier part left out)\n\n" + out[len(out)-maxTranscript:]
	}
	return out
}

func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + " …(cut)"
}

// gitState describes the checkout: its branch, the commits since the run began,
// and what is not committed yet.
func gitState(ctx context.Context, git GitFunc, dir, since string) string {
	var b strings.Builder
	section := func(title string, args ...string) {
		out, err := git(ctx, dir, args...)
		out = strings.TrimSpace(out)
		if err != nil || out == "" {
			out = "(none)"
		}
		fmt.Fprintf(&b, "%s:\n```\n%s\n```\n\n", title, out)
	}
	section("Branch", "rev-parse", "--abbrev-ref", "HEAD")
	if since != "" {
		section("Commits since the run began", "log", "--oneline", since+"..HEAD")
	}
	section("Uncommitted changes", "status", "--short")
	section("Diff summary", "diff", "--stat", "HEAD")
	return b.String()
}

// handoff is what the next harness reads first.
func handoff(from, to, state, transcript string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Handoff from %s to %s\n\n", from, to)
	fmt.Fprintf(&b, "%s stopped at its usage limit in the middle of this work. You "+
		"continue it in the same checkout. Check the git state yourself before acting: "+
		"it is the truth, the conversation below is context.\n\n", from)
	b.WriteString("## Git state\n\n" + state)
	if transcript != "" {
		b.WriteString("## Conversation so far (compacted)\n\n" + transcript)
	}
	return b.String()
}

// handoffPrompt points the next harness at the handoff file.
func handoffPrompt() string {
	return "Read " + vm.SandboxSessionDir + "/" + HandoffFile +
		" first: another agent stopped at its usage limit and you continue its work."
}
