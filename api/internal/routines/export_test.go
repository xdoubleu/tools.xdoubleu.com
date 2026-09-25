package routines

import "net/http"

// NewClientForTest builds a Client with any recorder; test-only, so
// client_test.go can stay in routines_test.
func NewClientForTest(baseURL, token string, recorder automatedActionRecorder) *Client {
	return &Client{
		BaseURL:          baseURL,
		Token:            token,
		HTTPClient:       http.DefaultClient,
		automatedActions: recorder,
	}
}
