package mocks

import (
	"context"
	"fmt"

	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
)

// MockWebFetchClient is a configurable in-memory webfetch.Client keyed by
// exact URL.
type MockWebFetchClient struct {
	// Responses maps URL -> result. URLs not present return an error wrapping
	// webfetch.ErrStatus (as a real 404 would).
	Responses map[string]*webfetch.Result
	// Errs maps URL -> forced error, taking precedence over Responses.
	Errs map[string]error
	// Calls records every requested URL in order.
	Calls []string
}

// NewMockWebFetchClient returns an empty mock (every URL 404s).
func NewMockWebFetchClient() *MockWebFetchClient {
	return &MockWebFetchClient{
		Responses: map[string]*webfetch.Result{},
		Errs:      map[string]error{},
		Calls:     nil,
	}
}

// SetHTML registers an HTML page response for url.
func (m *MockWebFetchClient) SetHTML(url, html string) {
	//nolint:exhaustruct // validators/NotModified unused in mock responses
	m.Responses[url] = &webfetch.Result{
		Body:        []byte(html),
		ContentType: "text/html",
		FinalURL:    url,
	}
}

// SetBody registers a raw response with the given content type for url.
func (m *MockWebFetchClient) SetBody(url, contentType string, body []byte) {
	//nolint:exhaustruct // validators/NotModified unused in mock responses
	m.Responses[url] = &webfetch.Result{
		Body:        body,
		ContentType: contentType,
		FinalURL:    url,
	}
}

// SetNotModified registers a 304 (conditional GET short-circuit) response for
// url — Body empty, NotModified true.
func (m *MockWebFetchClient) SetNotModified(url string) {
	//nolint:exhaustruct // a 304 carries no body or content type
	m.Responses[url] = &webfetch.Result{
		FinalURL:    url,
		NotModified: true,
	}
}

func (m *MockWebFetchClient) Get(
	_ context.Context,
	rawURL string,
	_ webfetch.Options,
) (*webfetch.Result, error) {
	m.Calls = append(m.Calls, rawURL)
	if err, ok := m.Errs[rawURL]; ok {
		return nil, err
	}
	res, ok := m.Responses[rawURL]
	if !ok {
		return nil, fmt.Errorf("%w: 404 (mock has no %s)", webfetch.ErrStatus, rawURL)
	}
	return res, nil
}
