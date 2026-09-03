package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
)

const testUserID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

func connectServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(testApp.Routes())
	t.Cleanup(ts.Close)
	return ts
}

// doInProcess executes a request directly against the handler using
// httptest.NewRecorder so that it hits the 192.0.2.1 rate-limit bucket
// (not the 127.0.0.1 bucket consumed by httptest.NewServer-based tests).
func doInProcess(
	t *testing.T,
	method, target string,
	body string,
	contentType string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody strings.Reader
	if body != "" {
		reqBody = *strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, &reqBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)
	return rr
}

func setCookieOnRequest[T any](req *connect.Request[T], cookies ...http.Cookie) {
	var parts []string
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	req.Header().Set("Cookie", strings.Join(parts, "; "))
}

func promoteToAdmin(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.appUsersRepo.SetRole(ctx, testUserID, models.RoleAdmin))
}

func demoteToUser(t *testing.T) {
	t.Helper()
	require.NoError(t,
		testApp.appUsersRepo.SetRole(context.Background(), testUserID, models.RoleUser))
}

func grantAppAccess(t *testing.T, userID, appName string) {
	t.Helper()
	require.NoError(t, testApp.appUsersRepo.SetAppAccess(
		context.Background(), userID, appName, true,
	))
}

func revokeAppAccess(t *testing.T, userID, appName string) {
	t.Helper()
	require.NoError(t, testApp.appUsersRepo.SetAppAccess(
		context.Background(), userID, appName, false,
	))
}
