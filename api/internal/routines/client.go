// Package routines fires a Claude Code routine's webhook so a Grafana alert
// can trigger it directly. The endpoint shape (POST <base>/<name>/fire, bearer
// auth, {"text": ...}) is an unverified guess and currently 404s; errors carry
// the posted URL and response body to help find the real contract.
package routines

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/repositories"
)

var errEmptyRoutineName = errors.New("routine name is required")

// defaultTimeout keeps a third-party webhook from hanging the caller.
const defaultTimeout = 10 * time.Second

type automatedActionRecorder interface {
	Open(ctx context.Context, triggerSource, routineName string) (int64, error)
	Close(ctx context.Context, id int64, outcome, prURL, errorText string) error
}

// Client fires a routine webhook, recording the attempt in
// global.automated_actions first so it exists even if the call fails. The
// routine closes the row itself; if the call fails, Fire closes it.
type Client struct {
	// BaseURL is the webhook base; Fire posts to <BaseURL>/<routineName>/fire.
	BaseURL string
	// Token is the outbound bearer token.
	Token string
	// HTTPClient defaults to a client with defaultTimeout.
	HTTPClient *http.Client

	automatedActions automatedActionRecorder
}

// NewClient builds a Client that records runs with trigger source "api".
func NewClient(
	baseURL, token string, automatedActions *repositories.AutomatedActionsRepository,
) *Client {
	return &Client{
		BaseURL:          baseURL,
		Token:            token,
		HTTPClient:       &http.Client{Timeout: defaultTimeout},
		automatedActions: automatedActions,
	}
}

type firePayload struct {
	// Text is freeform context so the routine needn't rediscover the trigger.
	Text string `json:"text,omitempty"`
}

// Fire records the trigger, then POSTs text to the routine's webhook. On
// failure it closes the row as failed, since the routine never ran.
func (c *Client) Fire(ctx context.Context, routineName, text string) error {
	if routineName == "" {
		return errEmptyRoutineName
	}

	id, err := c.automatedActions.Open(ctx, "api", routineName)
	if err != nil {
		return fmt.Errorf(
			"recording automated action for routine %q: %w",
			routineName,
			err,
		)
	}

	if fireErr := c.postFire(ctx, routineName, text); fireErr != nil {
		return c.closeStranded(ctx, id, routineName, fireErr)
	}

	return nil
}

func (c *Client) postFire(ctx context.Context, routineName, text string) error {
	// Marshaling a single string field cannot fail.
	body, _ := json.Marshal(firePayload{Text: text})

	url := strings.TrimRight(c.BaseURL, "/") + "/" + routineName + "/fire"
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, url, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("building fire request for routine %q: %w", routineName, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("firing routine %q: %w", routineName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		// The contract is undocumented, so fold the URL and response body into the
		// error for diagnosis.
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"firing routine %q: unexpected status %d from %s: %s",
			routineName, resp.StatusCode, url, string(raw),
		)
	}

	return nil
}

// closeStranded fails the row of a routine that was never invoked. It uses a
// context detached from ctx's cancellation (likely why postFire failed); if
// closing fails too, both errors are returned and AutomatedActionStalled
// catches the row.
func (c *Client) closeStranded(
	ctx context.Context, id int64, routineName string, fireErr error,
) error {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultTimeout)
	defer cancel()

	if closeErr := c.automatedActions.Close(
		closeCtx, id, "failed", "", fireErr.Error(),
	); closeErr != nil {
		return errors.Join(fireErr, fmt.Errorf(
			"closing stranded automated action %d for routine %q: %w",
			id, routineName, closeErr,
		))
	}

	return fireErr
}
