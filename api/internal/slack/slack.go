// Package slack posts the factory's notifications to a Slack incoming webhook.
package slack

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"tools.xdoubleu.com/internal/httpclient"
)

// postTimeout bounds one notification, retries included.
const (
	postTimeout  = 20 * time.Second
	sendAttempts = 3
)

// Escape makes text safe to put in a Slack message: Slack reads <...> as a link or
// a mention (<!channel>), so the brackets and & are escaped.
func Escape(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
}

// Sender posts text to one webhook. It is safe for concurrent use.
type Sender struct {
	url  string
	http *httpclient.Client
	log  *slog.Logger
	wg   sync.WaitGroup
}

// New returns a Sender for the webhook, or nil when the address is empty or is not
// https: the address is a secret, and a plain-http one would send it in the clear.
func New(webhookURL string, logger *slog.Logger) *Sender {
	return newSender(webhookURL, logger, nil)
}

// newSender is New with the HTTP client to send through; nil is the default.
func newSender(webhookURL string, logger *slog.Logger, hc *http.Client) *Sender {
	if !strings.HasPrefix(webhookURL, "https://") {
		return nil
	}
	return &Sender{
		url: webhookURL,
		http: httpclient.New(httpclient.Config{
			Name: "slack", Logger: logger, MaxAttempts: sendAttempts, HTTPClient: hc,
		}),
		log: logger,
		wg:  sync.WaitGroup{},
	}
}

// Notify posts text without making the caller wait: a notification must never slow
// or fail the work it reports. A failure is logged without the webhook address.
func (s *Sender) Notify(ctx context.Context, text string) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return
	}
	ctx = context.WithoutCancel(ctx)
	s.wg.Go(func() {
		sendCtx, cancel := context.WithTimeout(ctx, postTimeout)
		defer cancel()
		req := httpclient.NewRequest(http.MethodPost, s.url).
			SetHeader("Content-Type", "application/json").SetBody(body)
		resp, postErr := s.http.Do(sendCtx, req)
		switch {
		case postErr != nil:
			s.log.Warn("could not post to Slack", "error", postErr.Error())
		case resp.StatusCode >= http.StatusMultipleChoices:
			s.log.Warn("Slack refused a notification", "status", resp.StatusCode)
		}
	})
}

// Wait blocks until every notification handed to Notify has been tried.
func (s *Sender) Wait() { s.wg.Wait() }
