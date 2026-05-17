package testhelper

import "net/http"

// Routable is the interface all apps satisfy for route registration.
type Routable interface {
	Routes(prefix string, mux *http.ServeMux)
	GetName() string
}

// BuildMux creates an http.ServeMux with the app's routes registered.
func BuildMux(a Routable) http.Handler {
	mux := http.NewServeMux()
	a.Routes(a.GetName(), mux)
	return mux
}
