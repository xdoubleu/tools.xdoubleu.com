package main

import (
	"bufio"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const (
	usageGlobalApp     = "global"
	usageRootEndpoint  = "root"
	usageIDPlaceholder = ":id"
	maxEndpointLen     = 64
	maxPlainSegmentLen = 32
)

// usageMiddleware counts requests per app and endpoint. Must run after
// domainMiddleware.
func (app *Application) usageMiddleware(next http.Handler) http.Handler {
	appNames := make(map[string]bool)
	for _, a := range *app.apps {
		appNames[a.GetName()] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appName, endpoint, ok := usageLabels(r, appNames)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		counter := &countingResponseWriter{ResponseWriter: w, written: 0}
		next.ServeHTTP(counter, r)
		app.usage.Record(appName, endpoint, counter.written)
	})
}

// countingResponseWriter totals response bytes for usage_daily. It forwards
// Flush and Hijack, which GetDeployLogs streaming and WebSockets need.
type countingResponseWriter struct {
	http.ResponseWriter
	written int64
}

func (w *countingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func (w *countingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *countingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *countingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// usageLabels derives the (app, endpoint) counter labels for a request.
// Infrastructure endpoints and CORS preflights return ok=false.
func usageLabels(
	r *http.Request,
	appNames map[string]bool,
) (string, string, bool) {
	if r.Method == http.MethodOptions {
		return "", "", false
	}

	path := strings.Trim(r.URL.Path, "/")
	segments := strings.Split(path, "/")
	if path == "" || segments[0] == "health" || segments[0] == "api" {
		return "", "", false
	}

	appName := usageGlobalApp
	if appNames[segments[0]] {
		appName = segments[0]
		segments = segments[1:]
	} else if pkg, _, found := strings.Cut(segments[0], "."); found &&
		appNames[pkg] {
		// ConnectRPC paths carry no /<app>/ prefix; use the proto package.
		appName = pkg
	}

	return appName, endpointLabel(segments), true
}

// endpointLabel bounds cardinality: "Service/Method" for ConnectRPC, else the
// first path segment with IDs masked.
func endpointLabel(segments []string) string {
	if len(segments) == 0 || segments[0] == "" {
		return usageRootEndpoint
	}

	first := segments[0]
	if idx := strings.LastIndexByte(first, '.'); idx != -1 {
		service := first[idx+1:]
		if len(segments) > 1 {
			service += "/" + segments[1]
		}
		return truncate(service, maxEndpointLen)
	}

	if first == "oauth2" && len(segments) > 1 {
		return truncate("oauth2/"+segments[1], maxEndpointLen)
	}

	if _, err := uuid.Parse(first); err == nil ||
		len(first) > maxPlainSegmentLen {
		return usageIDPlaceholder
	}
	return truncate(first, maxEndpointLen)
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
