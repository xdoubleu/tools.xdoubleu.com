package kobo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"tools.xdoubleu.com/internal/communication/httptools"
)

// upstreamTimeout caps upstream Kobo store calls so a stalled store still
// leaves a sync time to answer with our own books.
const upstreamTimeout = httptools.UpstreamTimeout

//nolint:gochecknoglobals // shared client, Timeout mutated only in tests
var upstreamClient = &http.Client{Timeout: upstreamTimeout}

// ErrBuildRequest marks a Proxy failure before the request left (a bug on
// our side, not an unavailable store).
var ErrBuildRequest = errors.New("kobo: build upstream request")

// Upstream holds a merged-upstream sync result and a diagnostic note about
// how the store call went. Note is empty on success; on failure it carries the
// upstream HTTP status (or error) and whether the device's Authorization was
// present to forward — never the credential value.
type Upstream struct {
	Items []json.RawMessage
	Hdrs  http.Header
	Note  string
}

// PagingHeaders are the upstream paging headers to copy onto the response.
// They only travel with a merged upstream page: a "continue" without one
// makes the device re-request the same page forever.
func (u Upstream) PagingHeaders() http.Header {
	if u.Note != "" {
		return nil
	}
	out := http.Header{}
	for _, hdr := range []string{HeaderSync, HeaderSyncMode, HeaderSyncToken} {
		if v := u.Hdrs.Get(hdr); v != "" {
			out.Set(hdr, v)
		}
	}
	return out
}

// forwardAuth copies the device's store credential from the arriving
// request onto the upstream call. Kobo identifies its account to the real
// store through the Authorization header on the device request; with none to
// forward the store answers 401 and the merge silently empties.
func forwardAuth(dst, src http.Header) {
	if v := src.Get("Authorization"); v != "" {
		dst.Set("Authorization", v)
	}
}

// UpstreamNote is the diagnostic for a failed upstream store call: the
// status and whether an Authorization header was present to forward.
func UpstreamNote(status int, src http.Header) string {
	auth := "Authorization forwarded"
	if src.Get("Authorization") == "" {
		auth = "no Authorization forwarded"
	}
	return fmt.Sprintf("upstream %d (%s)", status, auth)
}

// Proxy sends a device request this app doesn't own to the upstream store.
// koboPath is the request path without the token segment. The caller closes
// the response body.
func (p *Protocol) Proxy(
	ctx context.Context,
	method, koboPath, rawQuery string,
	hdr http.Header,
	body io.Reader,
) (*http.Response, error) {
	targetURL := p.d.StoreBaseURL + koboPath
	if rawQuery != "" {
		targetURL += "?" + rawQuery
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBuildRequest, err)
	}
	req.Header = hdr.Clone()
	// Go then decompresses gzip itself, so the device and the debug capture
	// get plain bodies; the device's value would pass brotli through.
	req.Header.Del("Accept-Encoding")
	forwardAuth(req.Header, hdr)

	return upstreamClient.Do(req)
}

// fetchUpstreamSync returns upstream sync items (empty on failure) and
// headers, with a diagnostic note for the Kobo debug log when the store call
// fails or is unauthenticated.
func (p *Protocol) fetchUpstreamSync(
	ctx context.Context,
	rawQuery string,
	hdr http.Header,
) Upstream {
	targetURL := p.d.StoreBaseURL + "/v1/library/sync"
	if rawQuery != "" {
		targetURL += "?" + rawQuery
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		//nolint:exhaustruct // failure: items/hdrs stay nil
		return Upstream{Note: "upstream: build request failed"}
	}
	req.Header = hdr.Clone()
	// Go only decompresses gzip transparently when it set Accept-Encoding
	// itself; the device's value would leave the body compressed.
	req.Header.Del("Accept-Encoding")
	forwardAuth(req.Header, hdr)

	resp, err := upstreamClient.Do(req)
	if err != nil {
		//nolint:exhaustruct // failure: items/hdrs stay nil
		return Upstream{Note: "upstream: " + err.Error()}
	}
	defer resp.Body.Close()

	hdrs := resp.Header.Clone()
	if resp.StatusCode != http.StatusOK {
		//nolint:exhaustruct // failure: items stay nil
		return Upstream{Hdrs: hdrs, Note: UpstreamNote(resp.StatusCode, hdr)}
	}

	var items []json.RawMessage
	if decErr := json.NewDecoder(resp.Body).Decode(&items); decErr != nil {
		//nolint:exhaustruct // failure: items stay nil
		return Upstream{Hdrs: hdrs, Note: "upstream: decode failed"}
	}
	return Upstream{Items: items, Hdrs: hdrs, Note: ""}
}
