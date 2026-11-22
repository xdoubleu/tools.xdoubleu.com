package ward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func TestHandoffHeaderRoundTrips(t *testing.T) {
	h := Header{Version: HandoffVersion, From: "claude", To: "opencode", Base: "abc123"}
	text := handoff(h, "Branch:\n```\nx\n```\n\n", conversation{turns: nil, todos: nil})
	got, err := ParseHandoff(text)
	require.NoError(t, err)
	assert.Equal(t, h, got)
	assert.Contains(t, text, "\n# Handoff from claude to opencode")
}

func TestParseHandoffRejectsBadHeaders(t *testing.T) {
	for name, text := range map[string]string{
		"none":       "# Handoff",
		"unclosed":   "---\nwardcase-handoff: 1\n",
		"no version": "---\nfrom: a\n---\n",
		"bad":        "---\nwardcase-handoff: x\n---\n",
		"newer":      "---\nwardcase-handoff: 99\n---\n",
	} {
		_, err := ParseHandoff(text)
		assert.Error(t, err, name)
	}
}

func TestParseHandoffIgnoresUnknownKeys(t *testing.T) {
	h, err := ParseHandoff("---\nwardcase-handoff: 1\nfrom: a\nextra: z\n---\n\nbody")
	require.NoError(t, err)
	assert.Equal(t, "a", h.From)
}

func TestCompactKeepsTheTailOnTurnBoundaries(t *testing.T) {
	long := strings.Repeat("x", maxMessage+500)
	var c conversation
	for i := 0; i < 40; i++ {
		c.turns = append(c.turns, turn{who: whoUser, text: long})
	}
	c.turns = append(c.turns, turn{who: whoAgent, text: "last words"})
	out := c.compact()
	assert.True(t, strings.HasPrefix(out, truncationMarker))
	assert.Contains(t, out, "**Agent:** last words")
	assert.LessOrEqual(t, len(out), maxTranscript+len(truncationMarker)+2)
	assert.Contains(t, out, "**User:** x")
	assert.Contains(t, out, "…(cut)")
	body := strings.TrimPrefix(out, truncationMarker+"\n\n")
	assert.True(t, strings.HasPrefix(body, "**"))
}

func TestCompactLeavesShortConversationsAlone(t *testing.T) {
	c := conversation{turns: []turn{{whoUser, "hi"}}, todos: nil}
	assert.Equal(t, "**User:** hi\n\n", c.compact())
}

func TestCapTextKeepsCharactersWhole(t *testing.T) {
	out := capText("ééé", 3)
	assert.Equal(t, "é …(cut)", out)
}

func TestOpenTodosSkipsFinishedOnes(t *testing.T) {
	c := conversation{turns: nil, todos: []item{
		{"done", "completed"}, {"doing", "in_progress"}, {"later", "pending"},
		{" ", "pending"},
	}}
	assert.Equal(t, "- [~] doing\n- [ ] later\n", c.openTodos())
}

func TestClaudeConversationCarriesTodos(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "projects", "p", "s.jsonl"), strings.Join([]string{
		`{"type":"user","message":{"content":"go"}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"TodoWrite",` +
			`"input":{"todos":[{"content":"a","status":"completed"},` +
			`{"content":"b","status":"pending"}]}}]}}`,
	}, "\n"))
	c := conversationOf(Claude, dir)
	assert.Len(t, c.turns, 1)
	assert.Equal(t, "- [ ] b\n", c.openTodos())
}

func TestOpenCodeConversationReadsItsStorage(t *testing.T) {
	dir := t.TempDir()
	st := filepath.Join(dir, "opencode", "storage")
	write(t, filepath.Join(st, "session", "proj", "ses_1.json"), `{"id":"ses_1"}`)
	write(t, filepath.Join(st, "message", "ses_1", "msg_2.json"),
		`{"id":"msg_2","role":"assistant","time":{"created":20}}`)
	write(t, filepath.Join(st, "message", "ses_1", "msg_1.json"),
		`{"id":"msg_1","role":"user","time":{"created":10}}`)
	write(t, filepath.Join(st, "message", "ses_1", "msg_3.json"),
		`{"id":"msg_3","role":"system","time":{"created":5}}`)
	write(t, filepath.Join(st, "part", "msg_1", "prt_1.json"),
		`{"type":"text","text":"build it"}`)
	write(t, filepath.Join(st, "part", "msg_2", "prt_1.json"),
		`{"type":"tool","text":"no"}`)
	write(t, filepath.Join(st, "part", "msg_2", "prt_2.json"),
		`{"type":"text","text":"half"}`)
	write(t, filepath.Join(st, "part", "msg_2", "prt_3.json"),
		`{"type":"text","text":"synthetic","synthetic":true}`)
	write(t, filepath.Join(st, "todo", "ses_1.json"),
		`[{"content":"finish","status":"pending"}]`)

	c := conversationOf(OpenCode, dir)
	assert.Equal(t, []turn{{whoUser, "build it"}, {whoAgent, "half"}}, c.turns)
	assert.Equal(t, "- [ ] finish\n", c.openTodos())
}

func TestOpenCodeWithoutSessionsIsEmpty(t *testing.T) {
	c := conversationOf(OpenCode, t.TempDir())
	assert.Empty(t, c.turns)
	assert.Empty(t, conversationOf("codex", t.TempDir()).turns)
}

func TestOpenCodeToClaudeHandoffCarriesTheConversation(t *testing.T) {
	dir := t.TempDir()
	st := filepath.Join(dir, "opencode", "storage")
	write(t, filepath.Join(st, "session", "p", "ses_1.json"), `{}`)
	write(t, filepath.Join(st, "message", "ses_1", "m1.json"), `{"role":"user"}`)
	write(t, filepath.Join(st, "part", "m1", "p1.json"), `{"type":"text","text":"hello"}`)
	h := Header{Version: HandoffVersion, From: OpenCode, To: Claude, Base: ""}
	text := handoff(h, "state\n", conversationOf(OpenCode, dir))
	assert.Contains(t, text, "**User:** hello")
	_, err := ParseHandoff(text)
	require.NoError(t, err)
}
