// Package slack posts monitoring alerts to a Slack channel via an Incoming
// Webhook (https://api.slack.com/messaging/webhooks), the Slack side of the
// email/Slack delivery switch added in issue #1482. The webhook URL is
// configured through the monitoring settings UI and stored encrypted, not as
// a deploy secret, so it is passed to Send per call rather than baked into
// the client.
package slack

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

// ErrNotConfigured is returned when no webhook URL has been set. Callers
// treat it as a degraded (not failed) state, mirroring mailer.ErrNotConfigured.
var ErrNotConfigured = errors.New("slack: not configured")

const apiTimeout = 10 * time.Second

// Slack's block-kit limits: a header block's plain_text tops out at 150
// characters and a section block's mrkdwn text at 3000. We truncate a hair
// under 3000 to leave room for the notice.
const (
	headerMaxLen  = 150
	sectionMaxLen = 2900
)

// Client posts a subject/body pair to a Slack Incoming Webhook.
type Client interface {
	// Send posts to webhookURL. A "" webhookURL yields ErrNotConfigured.
	Send(ctx context.Context, webhookURL, subject, body string) error
}

type webhookClient struct {
	httpClient *http.Client
}

// New creates a Slack Incoming Webhook client.
func New() Client {
	return &webhookClient{httpClient: &http.Client{Timeout: apiTimeout}}
}

// block is one Slack Block Kit block (header or section).
type block struct {
	Type string    `json:"type"`
	Text blockText `json:"text"`
}

type blockText struct {
	Type string `json:"type"` // "plain_text" | "mrkdwn"
	Text string `json:"text"`
}

type webhookPayload struct {
	// Text is the notification-fallback string shown in Slack's list view
	// and push notifications; blocks render the message body itself.
	Text   string  `json:"text"`
	Blocks []block `json:"blocks"`
}

func (c *webhookClient) Send(
	ctx context.Context, webhookURL, subject, body string,
) error {
	if webhookURL == "" {
		return ErrNotConfigured
	}

	payload, err := json.Marshal(webhookPayload{
		Text: truncate(subject, headerMaxLen),
		Blocks: []block{
			{
				Type: "header",
				Text: blockText{
					Type: "plain_text",
					Text: truncate(subject, headerMaxLen),
				},
			},
			{
				Type: "section",
				Text: blockText{
					Type: "mrkdwn",
					Text: truncateBody(body),
				},
			},
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, webhookURL, bytes.NewReader(payload),
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

// truncate shortens s to at most maxLen runes, appending an ellipsis when it
// had to cut.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

// truncateBody caps a section body at Slack's block limit, replacing the
// tail with a pointer to the full content rather than a bare ellipsis.
func truncateBody(body string) string {
	const notice = "\n\n… (truncated — see the email or the monitoring dashboard)"
	runes := []rune(body)
	if len(runes) <= sectionMaxLen {
		return body
	}
	return string(runes[:sectionMaxLen]) + notice
}
