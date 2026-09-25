// Package slackwebhook posts text messages to a Slack Incoming Webhook,
// server-side, for the notify_slack MCP tool.
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

// ErrNotConfigured means the webhook URL is unset; callers degrade.
var ErrNotConfigured = errors.New("slackwebhook: not configured")

const apiTimeout = 10 * time.Second

// Client posts messages to a configured Slack Incoming Webhook.
type Client interface {
	// Send posts message, with a non-empty title bolded on its own line above.
	Send(ctx context.Context, title, message string) error
}

type webhookClient struct {
	httpClient *http.Client
	webhookURL string
}

// New creates a webhook client; an empty URL makes Send return
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
