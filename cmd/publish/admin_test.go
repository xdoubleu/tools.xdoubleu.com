package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/cmd/publish/internal/dtos"
	"tools.xdoubleu.com/internal/models"
)

const testUserID = "4001e9cf-3fbe-4b09-863f-bd1654cfbf76"

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

func TestAdminHandlerNonAdmin(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	demoteToUser(t)

	tReq := test.CreateRequestTester(testApp.Routes(), http.MethodGet, "/admin")
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/", rs.Header.Get("Location"))
}

func TestAdminHandlerAsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	tReq := test.CreateRequestTester(testApp.Routes(), http.MethodGet, "/admin")
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestAdminSetRoleHandler(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/admin/users/"+testUserID+"/role",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.SetRoleDto{Role: models.RoleUser})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, user.Role)
}

func TestAdminSetRoleHandlerInvalidRole(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/admin/users/"+testUserID+"/role",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.SetRoleDto{Role: "superuser"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnprocessableEntity, rs.StatusCode)
}

func TestAdminSetAppAccessHandler(t *testing.T) {
	ctx := context.Background()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/admin/users/"+testUserID+"/access/backlog",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(dtos.SetAppAccessDto{Grant: true})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)

	user, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.Contains(t, user.AppAccess, "backlog")

	tReq2 := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/admin/users/"+testUserID+"/access/backlog",
	)
	tReq2.AddCookie(&accessToken)
	tReq2.SetFollowRedirect(false)
	tReq2.SetContentType(test.FormContentType)
	tReq2.SetData(dtos.SetAppAccessDto{Grant: false})

	rs2 := tReq2.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs2.StatusCode)

	user2, err := testApp.appUsersRepo.GetByID(ctx, testUserID)
	require.NoError(t, err)
	assert.NotContains(t, user2.AppAccess, "backlog")
}

func TestAdminSetRoleHandlerHTMX(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	ts := httptest.NewServer(testApp.Routes())
	defer ts.Close()

	body := url.Values{"role": {"user"}}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		ts.URL+"/admin/users/"+testUserID+"/role",
		strings.NewReader(body.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&accessToken)

	rs, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer rs.Body.Close()

	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestAdminSetAppAccessHandlerHTMX(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	ts := httptest.NewServer(testApp.Routes())
	defer ts.Close()

	body := url.Values{"grant": {"true"}}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		ts.URL+"/admin/users/"+testUserID+"/access/backlog",
		strings.NewReader(body.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(&accessToken)

	rs, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer rs.Body.Close()

	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
