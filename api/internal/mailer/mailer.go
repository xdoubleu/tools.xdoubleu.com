// Package mailer sends notification emails via the Resend HTTP API
// (https://resend.com), used by the issue-notifier job (issue #561) to alert
// an admin when a new Sentry issue or failed DigitalOcean deployment appears.
package mailer

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

// ErrNotConfigured is returned when the Resend API key, from address, or
// recipient is unset. Callers treat it as a degraded (not failed) state.
var ErrNotConfigured = errors.New("mailer: not configured")

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://api.resend.com"

const apiTimeout = 10 * time.Second

// Client sends notification emails.
type Client interface {
	// Send sends to the fixed recipient configured via New.
	Send(ctx context.Context, subject, body string) error
	// SendTo sends to an arbitrary recipient, independent of the recipient
	// passed to New (e.g. a user-facing transactional email).
	SendTo(ctx context.Context, to, subject, body string) error
}

type resendClient struct {
	httpClient *http.Client
	apiKey     string
	from       string
	to         string
}

// New creates a Resend-backed mailer. apiKey, from and to are read from
// config; if any is empty, Send always returns ErrNotConfigured.
func New(apiKey, from, to string) Client {
	return &resendClient{
		httpClient: &http.Client{Timeout: apiTimeout},
		apiKey:     apiKey,
		from:       from,
		to:         to,
	}
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func (c *resendClient) Send(ctx context.Context, subject, body string) error {
	return c.sendTo(ctx, c.to, subject, body)
}

func (c *resendClient) SendTo(ctx context.Context, to, subject, body string) error {
	return c.sendTo(ctx, to, subject, body)
}

func (c *resendClient) sendTo(ctx context.Context, to, subject, body string) error {
	if c.apiKey == "" || c.from == "" || to == "" {
		return ErrNotConfigured
	}

	payload, err := json.Marshal(sendRequest{
		From:    c.from,
		To:      []string{to},
		Subject: subject,
		Text:    body,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"/emails", bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API returned %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

// SetBaseURL overrides the Resend API base URL. Intended for tests only.
func SetBaseURL(u string) { baseURL = u }
