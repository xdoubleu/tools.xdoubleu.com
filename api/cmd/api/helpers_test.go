package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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

func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}

func insertPendingContact(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	senderUUID := uuid.New()
	senderID := senderUUID.String()
	senderEmail := "sender-" + senderID + "@example.com"

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, senderID, senderEmail))
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.contacts.AddByEmail(
		ctx, senderID, "user@example.com", "Test Sender",
	))

	incoming, err := testApp.contacts.ListIncoming(ctx, testUserID)
	require.NoError(t, err)

	var contactID string
	for _, c := range incoming {
		if c.OwnerUserID == senderID {
			contactID = c.ID.String()
			break
		}
	}
	require.NotEmpty(t, contactID, "expected a pending contact request from sender")
	return contactID
}
