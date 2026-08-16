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

// usageMiddleware counts every request per app and endpoint for the admin
// dashboard. It must run after domainMiddleware so custom-domain requests
// are already rewritten to /<app>/… paths.
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

		counter := &countingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(counter, r)
		app.usage.Record(appName, endpoint, counter.written)
	})
}

// countingResponseWriter totals the response body bytes a handler writes, so
// usage_daily can show which endpoints move the most data and not just which
// are called most (issue #1027).
//
// It forwards Flush and Hijack rather than swallowing them: the api has one
// server-streaming RPC (GetDeployLogs) and WebSocket upgrades
// (internal/progressws), both of which break if the wrapper hides those
// interfaces from the underlying writer.
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

// Unwrap lets http.ResponseController reach the underlying writer for any
// capability not forwarded explicitly above.
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
	}

	return appName, endpointLabel(segments), true
}

// endpointLabel keeps counter cardinality bounded: ConnectRPC paths become
// "Service/Method", other paths keep only their first segment with IDs and
// tokens masked.
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
