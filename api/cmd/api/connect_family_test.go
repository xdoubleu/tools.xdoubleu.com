package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	familyv1 "tools.xdoubleu.com/gen/family/v1"
	"tools.xdoubleu.com/gen/family/v1/familyv1connect"
)

func familyClient(t *testing.T) familyv1connect.FamilyServiceClient {
	t.Helper()
	ts := connectServer(t)
	return familyv1connect.NewFamilyServiceClient(ts.Client(), ts.URL)
}

// insertPendingFamilyInvite seeds a fresh sender (app-user only, no login) who
// invites testUserID to their family, mirroring insertPendingContact's shape.
// Returns the sender's user ID.
func insertPendingFamilyInvite(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	senderID := uuid.New().String()
	senderEmail := "sender-" + senderID + "@example.com"

	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, senderID, senderEmail))
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.family.InviteByEmail(ctx, senderID, "user@example.com"))

	return senderID
}

func TestGetFamily_Unauthenticated(t *testing.T) {
	client := familyClient(t)
	_, err := client.GetFamily(
		context.Background(),
		connect.NewRequest(&familyv1.GetFamilyRequest{}),
	)
	require.Error(t, err)
}

func TestGetFamily_ImplicitSolo(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.family.Decline(ctx, testUserID))
	require.NoError(t, testApp.family.Leave(ctx, testUserID))

	client := familyClient(t)
	req := connect.NewRequest(&familyv1.GetFamilyRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetFamily(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Members)
	assert.Nil(t, resp.Msg.IncomingInvite)
}

func TestInviteToFamily_NotFound(t *testing.T) {
	client := familyClient(t)
	req := connect.NewRequest(&familyv1.InviteToFamilyRequest{
		Email: "nonexistent@nowhere.example",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.InviteToFamily(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestInviteToFamily_CannotInviteSelf(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))

	client := familyClient(t)
	req := connect.NewRequest(&familyv1.InviteToFamilyRequest{
		Email: "user@example.com",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.InviteToFamily(context.Background(), req)
	require.Error(t, err)
}

func TestGetFamily_ShowsIncomingInvite(t *testing.T) {
	senderID := insertPendingFamilyInvite(t)

	client := familyClient(t)
	req := connect.NewRequest(&familyv1.GetFamilyRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.GetFamily(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.IncomingInvite)
	assert.Equal(t, senderID, resp.Msg.IncomingInvite.FromUserId)
}

func TestAcceptFamilyInvite_Success(t *testing.T) {
	senderID := insertPendingFamilyInvite(t)

	client := familyClient(t)
	req := connect.NewRequest(&familyv1.AcceptFamilyInviteRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.AcceptFamilyInvite(context.Background(), req)
	require.NoError(t, err)

	membership, err := testApp.family.GetMembership(context.Background(), senderID)
	require.NoError(t, err)
	assert.Contains(t, membership.Members, testUserID)

	// Leave again so later tests in this package see testUserID back in an
	// implicit solo family.
	require.NoError(t, testApp.family.Leave(context.Background(), testUserID))
}

func TestDeclineFamilyInvite_Success(t *testing.T) {
	insertPendingFamilyInvite(t)

	client := familyClient(t)
	req := connect.NewRequest(&familyv1.DeclineFamilyInviteRequest{})
	setCookieOnRequest(req, accessToken)
	_, err := client.DeclineFamilyInvite(context.Background(), req)
	require.NoError(t, err)

	_, ok, err := testApp.family.GetIncomingInvite(context.Background(), testUserID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestLeaveFamily_Success(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))

	before, err := testApp.family.GetMembership(ctx, testUserID)
	require.NoError(t, err)

	client := familyClient(t)
	req := connect.NewRequest(&familyv1.LeaveFamilyRequest{})
	setCookieOnRequest(req, accessToken)
	_, err = client.LeaveFamily(context.Background(), req)
	require.NoError(t, err)

	after, err := testApp.family.GetMembership(ctx, testUserID)
	require.NoError(t, err)
	assert.NotEqual(t, before.FamilyID, after.FamilyID,
		"leaving should hand testUserID a fresh solo family")
}
