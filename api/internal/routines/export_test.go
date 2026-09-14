package routines

import "net/http"

// NewClientForTest builds a Client backed by an arbitrary
// automatedActionOpener (typically a test fake), bypassing NewClient's
// concrete *repositories.AutomatedActionsRepository parameter. Only
// visible to tests — this file's _test.go suffix excludes it from the
// non-test build — which is what lets client_test.go live in the
// routines_test package (satisfying the testpackage linter) while still
// reaching Client's unexported automatedActions field.
func NewClientForTest(baseURL, token string, opener automatedActionOpener) *Client {
	return &Client{
		BaseURL:          baseURL,
		Token:            token,
		HTTPClient:       http.DefaultClient,
		automatedActions: opener,
	}
}
