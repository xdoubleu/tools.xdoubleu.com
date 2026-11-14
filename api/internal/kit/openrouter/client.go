// Package openrouter is a minimal OpenRouter chat-completions client
// (docs/adr-0032-openrouter-llm-for-taste.md).
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// ErrNotConfigured is returned when no API key is set.
var ErrNotConfigured = errors.New("openrouter: OPENROUTER_API_KEY is not set")

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://openrouter.ai/api/v1"

// requestTimeout bounds one completion, web searches included; callers run
// it from jobs, never a request handler (adr-0017).
const requestTimeout = 4 * time.Minute

const maxErrorBody = 2048

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// System is a system message.
func System(content string) Message { return Message{Role: "system", Content: content} }

// User is a user message.
func User(content string) Message { return Message{Role: "user", Content: content} }

// WebSearch enables OpenRouter's server-side web search tool, capped to bound
// its per-result cost.
type WebSearch struct {
	MaxResults      int
	MaxTotalResults int
}

type Request struct {
	Model    string
	Messages []Message
	// SchemaName and Schema, when set, require a JSON answer matching Schema.
	SchemaName string
	Schema     map[string]any
	WebSearch  *WebSearch
	MaxTokens  int
}

type Citation struct {
	URL   string
	Title string
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	// Cost is in USD credits, as OpenRouter reports it.
	Cost float64
}

type Response struct {
	// Model is the model that actually answered.
	Model     string
	Content   string
	Citations []Citation
	Usage     Usage
}

type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}

type client struct {
	apiKey  string
	referer string
	http    *http.Client
	logger  *slog.Logger
}

// New returns a client sending requests as referer (the app's public URL);
// with an empty apiKey every call returns ErrNotConfigured.
func New(apiKey, referer string, logger *slog.Logger) Client {
	return &client{
		apiKey:  apiKey,
		referer: referer,
		http:    &http.Client{Timeout: requestTimeout},
		logger:  logger,
	}
}

type wireTool struct {
	Type       string         `json:"type"`
	Parameters map[string]int `json:"parameters"`
}

type wireRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
	Tools          []wireTool     `json:"tools,omitempty"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	Usage          map[string]any `json:"usage"`
}

type wireResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content     string `json:"content"`
			Annotations []struct {
				Type        string `json:"type"`
				URLCitation struct {
					URL   string `json:"url"`
					Title string `json:"title"`
				} `json:"url_citation"`
			} `json:"annotations"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *client) Complete(ctx context.Context, req Request) (*Response, error) {
	if c.apiKey == "" {
		return nil, ErrNotConfigured
	}

	body, err := json.Marshal(wireBody(req))
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", c.referer)
	httpReq.Header.Set("X-Title", "tools.xdoubleu.com")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return nil, fmt.Errorf("openrouter: status %d: %s", resp.StatusCode, msg)
	}

	var wire wireResponse
	if err = json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, fmt.Errorf("openrouter: decode: %w", err)
	}
	return c.toResponse(req.Model, wire)
}

func wireBody(req Request) wireRequest {
	//nolint:exhaustruct // optional fields set below
	w := wireRequest{
		Model:     req.Model,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
		Usage:     map[string]any{"include": true},
	}
	if req.Schema != nil {
		w.ResponseFormat = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name": req.SchemaName, "strict": true, "schema": req.Schema,
			},
		}
	}
	if req.WebSearch != nil {
		w.Tools = []wireTool{{
			Type: "openrouter:web_search",
			Parameters: map[string]int{
				"max_results":       req.WebSearch.MaxResults,
				"max_total_results": req.WebSearch.MaxTotalResults,
			},
		}}
	}
	return w
}

func (c *client) toResponse(model string, wire wireResponse) (*Response, error) {
	if wire.Error != nil {
		return nil, fmt.Errorf("openrouter: %s", wire.Error.Message)
	}
	if len(wire.Choices) == 0 {
		return nil, errors.New("openrouter: no choices returned")
	}
	msg := wire.Choices[0].Message
	var citations []Citation
	for _, a := range msg.Annotations {
		if a.Type == "url_citation" && a.URLCitation.URL != "" {
			citations = append(citations, Citation{
				URL: a.URLCitation.URL, Title: a.URLCitation.Title,
			})
		}
	}
	c.logger.Info("openrouter completion",
		slog.String("model", wire.Model),
		slog.Int("prompt_tokens", wire.Usage.PromptTokens),
		slog.Int("completion_tokens", wire.Usage.CompletionTokens),
		slog.Float64("cost", wire.Usage.Cost),
	)
	answered := wire.Model
	if answered == "" {
		answered = model
	}
	return &Response{
		Model:     answered,
		Content:   msg.Content,
		Citations: citations,
		Usage: Usage{
			PromptTokens:     wire.Usage.PromptTokens,
			CompletionTokens: wire.Usage.CompletionTokens,
			Cost:             wire.Usage.Cost,
		},
	}, nil
}
