package testhelper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// RequestTester is used to test a certain HTTP request. Ported from
// essentia's pkg/test.RequestTester (issue #912), trimmed to the subset
// actually used in this repo -- AddCookie and Do.
type RequestTester struct {
	handler http.Handler
	method  string
	path    string
	cookies []*http.Cookie
}

// CreateRequestTester creates a new [RequestTester].
func CreateRequestTester(handler http.Handler, method, path string) RequestTester {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	return RequestTester{
		handler: handler,
		method:  method,
		path:    path,
		cookies: []*http.Cookie{},
	}
}

// AddCookie adds a cookie to a [RequestTester]. Can be used multiple times
// for adding several cookies.
func (tReq *RequestTester) AddCookie(cookie *http.Cookie) {
	tReq.cookies = append(tReq.cookies, cookie)
}

// Do executes a [RequestTester] and returns the response.
func (tReq *RequestTester) Do(t *testing.T) *http.Response {
	t.Helper()

	ts := httptest.NewServer(tReq.handler)
	defer ts.Close()

	req, err := http.NewRequestWithContext(
		context.Background(),
		tReq.method,
		fmt.Sprintf("%s/%s", ts.URL, tReq.path),
		nil,
	)
	if err != nil {
		t.Fatalf("error when creating request: %v", err)
	}

	for _, cookie := range tReq.cookies {
		req.AddCookie(cookie)
	}

	req.Header.Set("Content-Type", "application/json")

	rs, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("error when making request: %v", err)
	}

	return rs
}
