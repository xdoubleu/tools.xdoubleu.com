package ward

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tools.xdoubleu.com/wardcase/vm"
)

const (
	// HandoffFile is the handoff's name in the next harness's session directory.
	HandoffFile = "wardcase-handoff.md"

	// HandoffVersion is the format version this package writes and reads.
	HandoffVersion = 1

	versionKey       = "wardcase-handoff"
	truncationMarker = "…(earlier part left out)"

	whoUser    = "User"
	whoAgent   = "Agent"
	whoSummary = "Summary of the earlier conversation"

	maxMessage    = 2000
	maxTranscript = 30000
	maxTodo       = 300
	// transcriptTail is how much of a transcript is read for its last reply.
	transcriptTail = 256 << 10
	maxLineBytes   = 16 << 20
	lineBufBytes   = 64 << 10
)

// turn is one message of a conversation.
type turn struct{ who, text string }

// item is one entry of an agent's task list.
type item struct{ Content, Status string }

// conversation is what a harness's session holds that a handoff carries.
type conversation struct {
	turns []turn
	todos []item
}

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

// todos are the open items of the entry's last TodoWrite call, if it made one.
func (e entry) todos() ([]item, bool) {
	var blocks []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Input struct {
			Todos []struct {
				Content string `json:"content"`
				Status  string `json:"status"`
			} `json:"todos"`
		} `json:"input"`
	}
	if json.Unmarshal(e.Message.Content, &blocks) != nil {
		return nil, false
	}
	var out []item
	found := false
	for _, b := range blocks {
		if b.Type != "tool_use" || b.Name != "TodoWrite" {
			continue
		}
		found, out = true, nil
		for _, t := range b.Input.Todos {
			out = append(out, item{Content: t.Content, Status: t.Status})
		}
	}
	return out, found
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

// claudeConversation is the conversation of the session's latest Claude Code
// transcript from Claude's own last compact summary on, and its open todos.
func claudeConversation(sessionDir string) conversation {
	path := latestTranscript(sessionDir)
	if path == "" {
		return conversation{turns: nil, todos: nil}
	}
	entries := readEntries(path)
	start := 0
	for i, e := range entries {
		if e.IsCompactSummary {
			start = i
		}
	}
	var c conversation
	for _, e := range entries {
		if t, ok := e.todos(); ok {
			c.todos = t
		}
	}
	for _, e := range entries[start:] {
		text := strings.TrimSpace(e.text())
		if text == "" || e.IsMeta {
			continue
		}
		who := whoAgent
		switch {
		case e.IsCompactSummary:
			who = whoSummary
		case e.Type == "user":
			who = whoUser
		}
		c.turns = append(c.turns, turn{who: who, text: text})
	}
	return c
}

// conversationOf reads the named harness's latest session in its session
// directory; harnesses it cannot read give an empty conversation.
func conversationOf(name, sessionDir string) conversation {
	switch name {
	case Claude:
		return claudeConversation(sessionDir)
	case OpenCode:
		return openCodeConversation(sessionDir)
	default:
		return conversation{turns: nil, todos: nil}
	}
}

// compact is the conversation as the handoff's Conversation section: each turn
// capped, the whole kept to its most recent whole turns.
func (c conversation) compact() string {
	parts := make([]string, len(c.turns))
	total := 0
	for i, t := range c.turns {
		parts[i] = fmt.Sprintf("**%s:** %s\n\n", t.who, capText(t.text, maxMessage))
		total += len(parts[i])
	}
	first := 0
	for total > maxTranscript && first < len(parts)-1 {
		total -= len(parts[first])
		first++
	}
	out := strings.Join(parts[first:], "")
	if first > 0 {
		out = truncationMarker + "\n\n" + out
	}
	return out
}

// openTodos is the todos not yet completed, one checklist line each.
func (c conversation) openTodos() string {
	var b strings.Builder
	for _, t := range c.todos {
		if t.Status == "completed" || t.Status == "cancelled" ||
			strings.TrimSpace(t.Content) == "" {
			continue
		}
		mark := "[ ]"
		if t.Status == "in_progress" {
			mark = "[~]"
		}
		fmt.Fprintf(&b, "- %s %s\n", mark, capText(strings.TrimSpace(t.Content), maxTodo))
	}
	return b.String()
}

// capText cuts s to at most n bytes without splitting a character.
func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
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

// Header is a handoff's front matter (see HANDOFF.md).
type Header struct {
	Version int
	From    string
	To      string
	// Base is the commit the run began at, or "".
	Base string
}

func (h Header) String() string {
	return fmt.Sprintf("---\n%s: %d\nfrom: %s\nto: %s\nbase: %s\n---\n\n",
		versionKey, h.Version, h.From, h.To, h.Base)
}

// ParseHandoff reads a handoff's front matter. It fails on a missing header or a
// version newer than this one; unknown keys are ignored.
func ParseHandoff(text string) (Header, error) {
	var h Header
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return h, errors.New("handoff: no front matter")
	}
	block, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return h, errors.New("handoff: front matter is not closed")
	}
	for _, line := range strings.Split(block, "\n") {
		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case versionKey:
			n, err := strconv.Atoi(val)
			if err != nil {
				return h, fmt.Errorf("handoff: bad version %q", val)
			}
			h.Version = n
		case "from":
			h.From = val
		case "to":
			h.To = val
		case "base":
			h.Base = val
		}
	}
	if h.Version == 0 {
		return h, errors.New("handoff: no version")
	}
	if h.Version > HandoffVersion {
		return h, fmt.Errorf("handoff: version %d is newer than %d", h.Version,
			HandoffVersion)
	}
	return h, nil
}

// handoff is what the next harness reads first.
func handoff(h Header, state string, c conversation) string {
	var b strings.Builder
	b.WriteString(h.String())
	fmt.Fprintf(&b, "# Handoff from %s to %s\n\n", h.From, h.To)
	fmt.Fprintf(&b, "%s stopped at its usage limit in the middle of this work. You "+
		"continue it in the same checkout. Check the git state yourself before acting: "+
		"it is the truth, the conversation below is context.\n\n", h.From)
	b.WriteString("## Git state\n\n" + state)
	if todos := c.openTodos(); todos != "" {
		b.WriteString("## Open todos\n\n" + todos + "\n")
	}
	if text := c.compact(); text != "" {
		b.WriteString("## Conversation so far (compacted)\n\n" + text)
	}
	return b.String()
}

// handoffPrompt points the next harness at the handoff file.
func handoffPrompt() string {
	return "Read " + vm.SandboxSessionDir + "/" + HandoffFile +
		" first: another agent stopped at its usage limit and you continue its work."
}
