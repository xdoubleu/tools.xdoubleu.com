// Package zitadel is the api's client for Zitadel's user API, used by user
// management to create, invite and deactivate people (adr-0051).
package zitadel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/config"
)

const (
	httpTimeout  = 15 * time.Second
	maxBodyBytes = 1 << 20
	// A Zitadel user needs both names; a person known only by an email gets
	// this as their family name.
	fallbackFamilyName = "-"
)

// Client calls Zitadel as a service user that may manage only human users.
type Client struct {
	base   string
	token  string
	client *http.Client
}

// New is nil when no issuer or service token is configured.
func New(cfg config.Config) *Client {
	if cfg.OIDCIssuer == "" || cfg.ZitadelServiceToken == "" {
		return nil
	}
	return &Client{
		base:   strings.TrimSuffix(cfg.OIDCIssuer, "/"),
		token:  cfg.ZitadelServiceToken,
		client: &http.Client{Timeout: httpTimeout},
	}
}

// CreateHuman creates an active human user whose email counts as verified, and
// returns its ID, the subject of its ID tokens. It sends no mail.
func (c *Client) CreateHuman(
	ctx context.Context,
	email, displayName string,
) (string, error) {
	given, family := splitName(displayName, email)
	body := map[string]any{
		"username": email,
		"profile": map[string]string{
			"givenName": given, "familyName": family, "displayName": displayName,
		},
		"email": map[string]any{"email": email, "isVerified": true},
	}
	var out struct {
		UserID string `json:"userId"`
	}
	if err := c.do(ctx, http.MethodPost, "/v2/users/human", body, &out); err != nil {
		return "", err
	}
	if out.UserID == "" {
		return "", errors.New("zitadel: create user returned no id")
	}
	return out.UserID, nil
}

// SendInvite has Zitadel mail the person a link to set their credentials.
func (c *Client) SendInvite(ctx context.Context, userID string) error {
	return c.do(
		ctx, http.MethodPost, userPath(userID, "/invite_code"),
		map[string]any{"sendCode": map[string]any{}}, nil,
	)
}

// Deactivate blocks the user's sign-in; a user that is already deactivated is
// not an error, so a retry after a partial failure succeeds.
func (c *Client) Deactivate(ctx context.Context, userID string) error {
	err := c.do(ctx, http.MethodPost, userPath(userID, "/deactivate"), nil, nil)
	var status *StatusError
	if errors.As(err, &status) && status.Code == http.StatusPreconditionFailed {
		return nil
	}
	return err
}

// Delete removes the user; a user that is already gone is not an error.
func (c *Client) Delete(ctx context.Context, userID string) error {
	err := c.do(ctx, http.MethodDelete, userPath(userID, ""), nil, nil)
	var status *StatusError
	if errors.As(err, &status) && status.Code == http.StatusNotFound {
		return nil
	}
	return err
}

// StatusError is a non-2xx answer from Zitadel.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("zitadel: status %d: %s", e.Code, e.Body)
}

func userPath(userID, suffix string) string {
	return "/v2/users/" + url.PathEscape(userID) + suffix
}

// splitName derives Zitadel's required given and family names.
func splitName(displayName, email string) (string, string) {
	fields := strings.Fields(displayName)
	switch len(fields) {
	case 0:
		local, _, _ := strings.Cut(email, "@")
		return local, fallbackFamilyName
	case 1:
		return fields[0], fallbackFamilyName
	default:
		return fields[0], strings.Join(fields[1:], " ")
	}
}

func (c *Client) do(
	ctx context.Context,
	method, path string,
	in, out any,
) error {
	payload := []byte("{}")
	if in != nil {
		var err error
		if payload, err = json.Marshal(in); err != nil {
			return err
		}
	}
	var reader io.Reader
	if method != http.MethodDelete {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &StatusError{Code: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}
