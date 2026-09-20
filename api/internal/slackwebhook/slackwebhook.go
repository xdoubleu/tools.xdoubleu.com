// Package slackwebhook posts simple text messages to a Slack Incoming
// Webhook. It backs the notify_slack MCP tool (issue #1628), which lets an
// agent session — local or Claude Code on the web — post an epic-complete
// summary to Slack without any session-side setup difference: the send
// happens server-side, in api. Deliberately not named internal/slack, which
// adr-0022 Phase 4 retired along with the old notification fan-out system.
package slackwebhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrNotConfigured is returned when the webhook URL is unset. Callers treat
// it as a degraded (not failed) state, mirroring internal/mailer and
// internal/github/internal/sentryapi's degrade-gracefully pattern.
var ErrNotConfigured = errors.New("slackwebhook: not configured")

const apiTimeout = 10 * time.Second

// Client posts messages to a configured Slack Incoming Webhook.
type Client interface {
	// Send posts message to the webhook, bolding title on its own line above
	// it when non-empty.
	Send(ctx context.Context, title, message string) error
}

type webhookClient struct {
	httpClient *http.Client
	webhookURL string
}

// New creates a Slack Incoming Webhook client. webhookURL is read from
// config (SLACK_WEBHOOK_URL); if empty, Send always returns
// ErrNotConfigured.
func New(webhookURL string) Client {
	return &webhookClient{
		httpClient: &http.Client{Timeout: apiTimeout},
		webhookURL: webhookURL,
	}
}

type sendRequest struct {
	Text string `json:"text"`
}

func (c *webhookClient) Send(ctx context.Context, title, message string) error {
	if c.webhookURL == "" {
		return ErrNotConfigured
	}

	text := message
	if title != "" {
		text = fmt.Sprintf("*%s*\n%s", title, message)
	}

	payload, err := json.Marshal(sendRequest{Text: text})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.webhookURL, bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"slack webhook returned %d: %s", resp.StatusCode, string(raw),
		)
	}
	return nil
}
