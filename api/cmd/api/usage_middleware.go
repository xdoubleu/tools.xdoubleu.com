package main

import (
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

// legacyAppPrefixes maps retired URL prefixes to the app that still serves
// them. Kobo devices registered before the books→reading rename keep calling
// /books/kobo/… forever, and those requests should count as reading traffic.
//
//nolint:gochecknoglobals,goconst // static lookup table
var legacyAppPrefixes = map[string]string{"books": "reading"}

// usageMiddleware counts every request per app and endpoint for the admin
// dashboard. It must run after domainMiddleware so custom-domain requests
// are already rewritten to /<app>/… paths.
func (app *Application) usageMiddleware(next http.Handler) http.Handler {
	appNames := make(map[string]bool)
	for _, a := range *app.apps {
		appNames[a.GetName()] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if appName, endpoint, ok := usageLabels(r, appNames); ok {
			app.usage.Record(appName, endpoint)
		}
		next.ServeHTTP(w, r)
	})
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
	} else if target, isLegacy := legacyAppPrefixes[segments[0]]; isLegacy {
		appName = target
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
