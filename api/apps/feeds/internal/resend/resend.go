// Package resend retrieves received emails from Resend's API.
package resend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"tools.xdoubleu.com/internal/platform/communication/httptools"
)

// DefaultBaseURL is Resend's production API.
const DefaultBaseURL = "https://api.resend.com"

//nolint:gochecknoglobals // overridable in tests
var baseURL = DefaultBaseURL

// SetBaseURL overrides the Resend API base URL. Tests only.
func SetBaseURL(u string) { baseURL = u }

// Client calls Resend's retrieve-received-email API.
type Client struct {
	apiKey string
	http   *http.Client
}

// New returns a Client authenticating with apiKey.
func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: httptools.UpstreamTimeout},
	}
}

// receivedEmail is the used subset of GET /emails/receiving/{id}.
type receivedEmail struct {
	HTML string `json:"html"`
	Text string `json:"text"`
}

// HTML retrieves a received email's body, falling back to the text part
// wrapped as paragraphs.
func (c *Client) HTML(ctx context.Context, emailID string) (string, error) {
	url := baseURL + "/emails/receiving/" + emailID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(
			"resend receiving API returned %d: %s", resp.StatusCode, string(raw),
		)
	}

	var received receivedEmail
	if err = json.NewDecoder(resp.Body).Decode(&received); err != nil {
		return "", err
	}
	if received.HTML != "" {
		return received.HTML, nil
	}
	if received.Text != "" {
		return "<pre>" + received.Text + "</pre>", nil
	}
	return "", errors.New("received email has no html or text body")
}
