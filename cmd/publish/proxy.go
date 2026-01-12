package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"tools.xdoubleu.com/cmd/publish/internal/dtos"
)

func (app *Application) Proxy(w http.ResponseWriter, r *http.Request) {
	if !app.checkAuth(w, r) {
		return
	}

	if r.Method == http.MethodConnect {
		handleHTTPS(w, r)
		return
	}

	handleHTTP(w, r)
}

func (app *Application) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" {
		unauthorized(w)
		return false
	}

	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		unauthorized(w)
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		unauthorized(w)
		return false
	}

	//nolint:mnd //2 is fine here
	parts := strings.SplitN(string(decoded), ":", 2)

	//nolint:mnd //2 is fine here
	if len(parts) != 2 {
		unauthorized(w)
		return false
	}

	_, _, err = app.services.Auth.SignInWithEmail(&dtos.SignInDto{
		Email:      parts[0],
		Password:   parts[1],
		RememberMe: true,
	})

	if err != nil {
		unauthorized(w)
		return false
	}

	return true
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="AI Proxy"`)
	w.WriteHeader(http.StatusProxyAuthRequired)
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	rewritten := rewriteGoogleQuery(r.URL)

	req, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		rewritten.String(),
		r.Body,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	req.Header = r.Header.Clone()
	req.Header.Del("Proxy-Authorization")

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleHTTPS(w http.ResponseWriter, r *http.Request) {
	//nolint:exhaustruct //fields are optional
	dialer := &net.Dialer{}
	destConn, err := dialer.DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n")

	go transfer(destConn, clientConn)
	go transfer(clientConn, destConn)
}

func transfer(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	_, err := io.Copy(dst, src)
	if err != nil {
		panic(err)
	}
}

func rewriteGoogleQuery(u *url.URL) *url.URL {
	if !strings.Contains(u.Host, "google.") || u.Path != "/search" {
		return u
	}

	q := u.Query()
	query := q.Get("q")

	if query == "" || strings.Contains(query, "-ai") {
		return u
	}

	q.Set("q", query+" -ai")
	u.RawQuery = q.Encode()

	return u
}

func copyHeader(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}
