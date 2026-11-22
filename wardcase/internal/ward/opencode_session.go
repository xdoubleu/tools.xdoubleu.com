package ward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// openCodeTime is the millisecond timestamps in OpenCode's session files.
type openCodeTime struct {
	Created int64 `json:"created"`
}

// openCodeConversation is the conversation of the latest session in OpenCode's
// JSON storage (`<XDG_DATA_HOME>/opencode/storage/`): user and assistant text
// parts in order, without tool calls, and its open todos. A session stored in
// OpenCode's SQLite database is not read.
func openCodeConversation(sessionDir string) conversation {
	storage := filepath.Join(sessionDir, "opencode", "storage")
	id := latestOpenCodeSession(storage)
	if id == "" {
		return conversation{turns: nil, todos: nil}
	}
	return conversation{
		turns: openCodeTurns(storage, id),
		todos: openCodeTodos(storage, id),
	}
}

// latestOpenCodeSession is the id of the session whose file was written last.
func latestOpenCodeSession(storage string) string {
	files, _ := filepath.Glob(filepath.Join(storage, "session", "*", "*.json"))
	var best string
	var bestNanos int64
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil && (best == "" || info.ModTime().UnixNano() > bestNanos) {
			best, bestNanos = f, info.ModTime().UnixNano()
		}
	}
	return strings.TrimSuffix(filepath.Base(best), ".json")
}

func readJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(data, v) == nil
}

func openCodeTurns(storage, session string) []turn {
	type message struct {
		ID   string       `json:"id"`
		Role string       `json:"role"`
		Time openCodeTime `json:"time"`
	}
	files, _ := filepath.Glob(filepath.Join(storage, "message", session, "*.json"))
	var msgs []message
	for _, f := range files {
		var m message
		if readJSON(f, &m) && (m.Role == roleUser || m.Role == roleAssistant) {
			if m.ID == "" {
				m.ID = strings.TrimSuffix(filepath.Base(f), ".json")
			}
			msgs = append(msgs, m)
		}
	}
	sort.SliceStable(msgs, func(i, j int) bool {
		if msgs[i].Time.Created != msgs[j].Time.Created {
			return msgs[i].Time.Created < msgs[j].Time.Created
		}
		return msgs[i].ID < msgs[j].ID
	})
	var out []turn
	for _, m := range msgs {
		text := openCodeText(storage, m.ID)
		if text == "" {
			continue
		}
		who := whoAgent
		if m.Role == roleUser {
			who = whoUser
		}
		out = append(out, turn{who: who, text: text})
	}
	return out
}

// openCodeText is a message's text parts in order.
func openCodeText(storage, message string) string {
	type part struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Text      string `json:"text"`
		Synthetic bool   `json:"synthetic"`
	}
	files, _ := filepath.Glob(filepath.Join(storage, "part", message, "*.json"))
	sort.Strings(files)
	var texts []string
	for _, f := range files {
		var p part
		if readJSON(f, &p) && p.Type == "text" && !p.Synthetic &&
			strings.TrimSpace(p.Text) != "" {
			texts = append(texts, strings.TrimSpace(p.Text))
		}
	}
	return strings.Join(texts, "\n")
}

func openCodeTodos(storage, session string) []item {
	var items []item
	readJSON(filepath.Join(storage, "todo", session+".json"), &items)
	return items
}
