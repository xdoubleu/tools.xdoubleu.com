package runner

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
)

const (
	// heartbeatTimeout stays above the 20 s hold and under nothing the server
	// enforces, so a stuck connection ends here.
	heartbeatTimeout = 35 * time.Second
	pairTimeout      = 15 * time.Second
	maxResponseBytes = 1 << 20
)

// Error is a Connect error answered by the server.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("server: %s: %s", e.Code, e.Message)
}

// Unauthenticated reports a rejected device token: the runner was revoked.
func Unauthenticated(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == "unauthenticated"
}

// Client calls the server's RunnerService over the Connect protocol's JSON
// encoding, so the module needs no generated code.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Version string
}

// NewClient returns a client for the API root server.
func NewClient(server, version string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(server, "/"),
		HTTP:    http.DefaultClient,
		Version: version,
	}
}

type pairRequest struct {
	Code    string `json:"code"`
	Version string `json:"version"`
}

type pairResponse struct {
	DeviceToken string `json:"deviceToken"`
	RunnerName  string `json:"runnerName"`
}

type heartbeatRequest struct {
	Version string `json:"version"`
}

// Pair trades a pairing code for the device token and the runner's name.
func (c *Client) Pair(ctx context.Context, code string) (string, string, error) {
	var resp pairResponse
	err := c.call(
		ctx,
		"Pair",
		"",
		pairTimeout,
		pairRequest{Code: code, Version: c.Version},
		&resp,
	)
	if err != nil {
		return "", "", err
	}
	if resp.DeviceToken == "" {
		return "", "", errors.New("server answered without a device token")
	}
	return resp.DeviceToken, resp.RunnerName, nil
}

// Heartbeat reports the runner alive. The server holds the call open for about
// 20 seconds before answering.
func (c *Client) Heartbeat(ctx context.Context, token string) error {
	return c.call(ctx, "Heartbeat", token, heartbeatTimeout,
		heartbeatRequest{Version: c.Version}, &struct{}{})
}

func (c *Client) call(
	ctx context.Context,
	method, token string,
	timeout time.Duration,
	in, out any,
) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("encode %s: %w", method, err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/factory.v1.RunnerService/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read %s: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		var e Error
		if json.Unmarshal(data, &e) == nil && e.Code != "" {
			return &e
		}
		return fmt.Errorf("%s: unexpected status %d", method, resp.StatusCode)
	}
	if err = json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", method, err)
	}
	return nil
}
