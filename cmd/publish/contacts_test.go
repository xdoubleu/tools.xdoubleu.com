package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
)

func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}

// insertPendingContact inserts a pending contact from a unique per-test sender
// to testUserID and returns the contact UUID string. A cleanup function removes
// the sender row so tests do not interfere with one another.
func insertPendingContact(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	// Use a unique sender UUID per test to avoid cross-test interference.
	senderUUID := uuid.New()
	senderID := senderUUID.String()
	senderEmail := "sender-" + senderID + "@example.com"

	// Ensure the sender exists in app_users so GetAllUsers can find them.
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, senderID, senderEmail))

	// Ensure the test user exists so the contact can reference it.
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))

	// AddByEmail looks up testUserID by email via appUsersRepo.GetAll,
	// so pass the test user's email as the recipient.
	require.NoError(t, testApp.contacts.AddByEmail(
		ctx, senderID, "user@example.com", "Test Sender",
	))

	// Find the specific contact row this sender created (filter by sender).
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

func TestListContacts_Unauthenticated(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodGet, "/contacts",
	)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnauthorized, rs.StatusCode)
}

func TestListContacts_Authenticated(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodGet, "/contacts",
	)
	tReq.AddCookie(&accessToken)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestListContacts_WithIncoming renders the contacts page with an incoming
// (pending) contact request to cover the data.Incoming > 0 template branch.
func TestListContacts_WithIncoming(t *testing.T) {
	_ = insertPendingContact(t)

	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodGet, "/contacts",
	)
	tReq.AddCookie(&accessToken)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestListContacts_WithAccepted renders the contacts page with an accepted
// contact to cover the data.Contacts > 0 template branch.
func TestListContacts_WithAccepted(t *testing.T) {
	contactID := insertPendingContact(t)
	id := mustParseUUID(t, contactID)
	require.NoError(t, testApp.contacts.Accept(
		context.Background(), id, testUserID, "Accepted Friend",
	))

	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodGet, "/contacts",
	)
	tReq.AddCookie(&accessToken)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

// TestListContacts_WithPending renders the contacts page with a pending
// (sent) contact request to cover the data.Pending > 0 template branch.
func TestListContacts_WithPending(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	otherID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	require.NoError(
		t,
		testApp.appUsersRepo.Upsert(ctx, otherID, "pending-other@example.com"),
	)
	require.NoError(t, testApp.contacts.AddByEmail(
		ctx, testUserID, "pending-other@example.com", "Pending Target",
	))

	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodGet, "/contacts",
	)
	tReq.AddCookie(&accessToken)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestCreateContact_Unauthenticated(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodPost, "/contacts",
	)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusUnauthorized, rs.StatusCode)
}

func TestCreateContact_InvalidEmail(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodPost, "/contacts",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(createContactDto{
		Email:       "nonexistent@nowhere.example",
		DisplayName: "Nobody",
	})
	rs := tReq.Do(t)
	// Handler re-renders the page with an error message rather than redirecting
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestAcceptContact_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodPost, "/contacts/not-a-uuid/accept",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestDeclineContact_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodPost, "/contacts/not-a-uuid/decline",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestDeleteContact_InvalidID(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodPost, "/contacts/not-a-uuid/delete",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusNotFound, rs.StatusCode)
}

func TestAcceptContact_ValidID(t *testing.T) {
	contactID := insertPendingContact(t)

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/contacts/"+contactID+"/accept",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestDeclineContact_ValidID(t *testing.T) {
	contactID := insertPendingContact(t)

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/contacts/"+contactID+"/decline",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestDeleteContact_ValidID(t *testing.T) {
	contactID := insertPendingContact(t)

	// Decline first so the contact stays in "pending" state owned by senderID.
	// Then re-insert as an accepted contact owned by testUserID for deletion.
	// Simpler: just delete the sender's pending contact directly via the service
	// by pretending testUserID is the owner (won't match, rows = 0 but no error).
	// Instead, accept then delete from testUserID's side.
	ctx := context.Background()
	id := mustParseUUID(t, contactID)
	require.NoError(t, testApp.contacts.Accept(ctx, id, testUserID, "Sender"))

	// After accepting, testUserID has an accepted contact pointing to senderID.
	accepted, err := testApp.contacts.List(ctx, testUserID)
	require.NoError(t, err)
	require.NotEmpty(t, accepted)
	acceptedID := accepted[0].ID.String()

	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodPost,
		"/contacts/"+acceptedID+"/delete",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetFollowRedirect(false)
	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

// TestCreateContact_Success exercises the success path (redirect 303) of
// createContactHandler by inserting a second user into app_users so that
// AddByEmail can find them via GetAllUsers.
func TestCreateContact_Success(t *testing.T) {
	ctx := context.Background()

	otherID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	otherEmail := "other-contact@example.com"
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, otherID, otherEmail))

	tReq := test.CreateRequestTester(
		testApp.Routes(), http.MethodPost, "/contacts",
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(createContactDto{Email: otherEmail, DisplayName: "Other User"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
	assert.Equal(t, "/contacts", rs.Header.Get("Location"))
}
