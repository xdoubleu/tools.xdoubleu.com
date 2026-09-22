// Package routines fires a Claude Code routine's webhook with freeform
// context, letting a Grafana-detected problem trigger the routine directly
// instead of routing through an email a human has to notice (issue #1444).
//
// ASSUMPTION: this repo has no access to Claude Code's routine-fire webhook
// documentation, since routines run on Anthropic's own scheduled-agent
// infrastructure rather than anything this codebase controls. The shape
// implemented here — POST <base URL>/<routine name>/fire, bearer-token
// auth, JSON body {"text": "..."} — is a reasonable, documented-as-a-guess
// external API call; revisit it against the real contract once that's
// confirmed.
//
// Issue #1798 confirmed live that this guess is wrong — every fire attempt
// gets back a 404 (also reported from a different angle by #1808) — but no
// doc, ADR, or comment anywhere in this repo records what the real
// endpoint shape should be instead, so there is nothing to correct *to*
// with real confidence; swapping in another unverified guess would only
// trade one unconfirmed URL for another. Until the real contract is known,
// postFire instead captures the exact URL it posted to and the response
// body Anthropic sent back, both folded into the returned/logged (and
// therefore Sentry-reported) error — the response body is the most likely
// source of an actual clue for whoever fixes this for real next.
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

// errEmptyRoutineName is returned by Fire when routineName is empty.
var errEmptyRoutineName = errors.New("routine name is required")

// defaultTimeout bounds the outbound fire call — this hits a third-party
// webhook, so it shouldn't be allowed to hang the caller (the inbound
// Grafana webhook handler) indefinitely.
const defaultTimeout = 10 * time.Second

// automatedActionRecorder is the subset of
// *repositories.AutomatedActionsRepository Client depends on — narrowed to
// make Client trivially fakeable in tests without a real database.
type automatedActionRecorder interface {
	Open(ctx context.Context, triggerSource, routineName string) (int64, error)
	Close(ctx context.Context, id int64, outcome, prURL, errorText string) error
}

// Client fires a Claude Code routine's webhook, recording the attempt in
// global.automated_actions before making the outbound call. Writing that
// row first — with code this codebase controls — is what makes the record
// trustworthy: it exists even if the outbound call below fails. When the
// outbound call succeeds, the eventual outcome (succeeded/failed/
// no_action_needed) comes back from the routine itself, via a later
// CloseAutomatedAction call it makes on its own — but when the outbound
// call itself fails, the routine was never invoked and can never make that
// call, so Fire closes the row out itself (issue #1725) rather than
// leaving it open forever.
type Client struct {
	// BaseURL is the routine-fire webhook's base URL; Fire posts to
	// <BaseURL>/<routineName>/fire.
	BaseURL string
	// Token authenticates the outbound call as a bearer token.
	Token string
	// HTTPClient defaults to a client with defaultTimeout if left nil.
	HTTPClient *http.Client

	automatedActions automatedActionRecorder
}

// NewClient builds a Client backed by the given automated_actions
// repository (trigger source "api" — this call always originates from
// api's own process, e.g. the inbound Grafana webhook, never a schedule or
// a human).
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

// firePayload is the JSON body POSTed to the routine-fire webhook.
type firePayload struct {
	// Text is optional freeform context for the routine — e.g. the Sentry
	// issue, the failing PR, the Grafana alert's rule name/labels/
	// annotations — so the routine doesn't have to rediscover what
	// triggered it.
	Text string `json:"text,omitempty"`
}

// Fire records that routineName was triggered (trigger source "api") and
// then POSTs text as that routine's context to its fire webhook. The
// automated_actions row is written before the outbound call is attempted,
// so a run is recorded even if the call below fails — see the Client
// doc comment. If the outbound call fails, Fire closes that same row out
// itself (outcome "failed") before returning, since a routine that was
// never successfully invoked can never close it on its own.
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

// postFire builds and sends the outbound POST to routineName's fire
// webhook.
func (c *Client) postFire(ctx context.Context, routineName, text string) error {
	// firePayload's only field is a plain string, which json.Marshal can
	// never fail to encode — no error path to handle or test here.
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
		// The response body is read on the error path only — on success
		// it's discarded unread, same as before. Anthropic's routine-fire
		// endpoint contract isn't documented anywhere this repo can reach
		// (see the package doc comment), so the URL actually posted to and
		// whatever body came back are the two concrete facts most likely
		// to help diagnose *why* next time, folded straight into the error
		// this function's caller logs (and therefore reports to Sentry).
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"firing routine %q: unexpected status %d from %s: %s",
			routineName, resp.StatusCode, url, string(raw),
		)
	}

	return nil
}

// closeStranded closes out an automated_actions row that Fire opened but
// could never hand off to the routine — the outbound call itself failed
// (fireErr), so the routine was never invoked and can never call
// CloseAutomatedAction on its own (issue #1725). Left open, such a row
// would stall indefinitely, noticed only hours later by
// AutomatedActionStalled. Uses a context detached from ctx's own
// cancellation (but not its values) plus a fresh timeout, so a
// canceled/timed-out request context — quite possibly the reason
// postFire just failed — doesn't also block this recovery call. If the
// close itself fails too, both errors are joined and returned, and the
// row is left open — the same pre-existing situation
// AutomatedActionStalled exists to catch.
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
