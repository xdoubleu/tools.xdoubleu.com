package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

// serve points baseURL at handler for the test.
func serve(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	old := baseURL
	baseURL = ts.URL
	t.Cleanup(func() { baseURL = old })
}

func TestComplete_NotConfigured(t *testing.T) {
	c := New("", "https://tools.example", logging.NewNopLogger())
	//nolint:exhaustruct // model only
	_, err := c.Complete(context.Background(), Request{Model: "m"})
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestComplete_SendsSchemaAndWebSearch(t *testing.T) {
	var got map[string]any
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		assert.Equal(t, "https://tools.example", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "tools.xdoubleu.com", r.Header.Get("X-Title"))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{
			"model": "served/model",
			"choices": [{"message": {"content": "{\"a\":1}", "annotations": [
				{"type": "url_citation",
				 "url_citation": {"url": "https://a.example", "title": "A"}},
				{"type": "url_citation", "url_citation": {"url": ""}},
				{"type": "other"}
			]}}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "cost": 0.002}
		}`))
	})

	c := New("key", "https://tools.example", logging.NewNopLogger())
	resp, err := c.Complete(context.Background(), Request{
		Model:      "asked/model",
		Messages:   []Message{{Role: "user", Content: "hi"}},
		SchemaName: "answer",
		Schema:     map[string]any{"type": "object"},
		WebSearch:  &WebSearch{MaxResults: 5, MaxTotalResults: 20},
		MaxTokens:  100,
	})
	require.NoError(t, err)

	assert.Equal(t, "asked/model", got["model"])
	assert.InDelta(t, 100, got["max_tokens"], 0)
	assert.Equal(t, map[string]any{"include": true}, got["usage"])
	format, ok := got["response_format"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "json_schema", format["type"])
	assert.Equal(t, map[string]any{
		"name": "answer", "strict": true, "schema": map[string]any{"type": "object"},
	}, format["json_schema"])
	assert.Equal(t, []any{map[string]any{
		"type": "openrouter:web_search",
		"parameters": map[string]any{
			"max_results": float64(5), "max_total_results": float64(20),
		},
	}}, got["tools"])

	assert.Equal(t, "served/model", resp.Model)
	assert.JSONEq(t, `{"a":1}`, resp.Content)
	assert.Equal(t, []Citation{{URL: "https://a.example", Title: "A"}}, resp.Citations)
	assert.Equal(t, Usage{PromptTokens: 10, CompletionTokens: 5, Cost: 0.002}, resp.Usage)
}

func TestComplete_PlainRequestOmitsOptionalFields(t *testing.T) {
	var got map[string]any
	serve(t, func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"choices": [{"message": {"content": "ok"}}]}`))
	})
	c := New("key", "", logging.NewNopLogger())
	//nolint:exhaustruct // a plain request
	resp, err := c.Complete(context.Background(), Request{Model: "m"})
	require.NoError(t, err)
	assert.Equal(t, "m", resp.Model, "falls back to the requested model")
	assert.Equal(t, "ok", resp.Content)
	assert.NotContains(t, got, "response_format")
	assert.NotContains(t, got, "tools")
	assert.NotContains(t, got, "max_tokens")
}

func TestComplete_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"http status", http.StatusTooManyRequests, `slow down`, "status 429: slow down"},
		{"api error", http.StatusOK, `{"error": {"message": "bad model"}}`, "bad model"},
		{"no choices", http.StatusOK, `{"choices": []}`, "no choices"},
		{"bad json", http.StatusOK, `{`, "decode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serve(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			c := New("key", "", logging.NewNopLogger())
			//nolint:exhaustruct // model only
			_, err := c.Complete(context.Background(), Request{Model: "m"})
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestComplete_TransportError(t *testing.T) {
	old := baseURL
	baseURL = "http://127.0.0.1:1"
	t.Cleanup(func() { baseURL = old })
	c := New("key", "", logging.NewNopLogger())
	//nolint:exhaustruct // model only
	_, err := c.Complete(context.Background(), Request{Model: "m"})
	assert.ErrorContains(t, err, "openrouter:")
}
