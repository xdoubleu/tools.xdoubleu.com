package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contactsv1 "tools.xdoubleu.com/gen/contacts/v1"
	"tools.xdoubleu.com/gen/contacts/v1/contactsv1connect"
)

func contactsClient(t *testing.T) contactsv1connect.ContactsServiceClient {
	t.Helper()
	ts := connectServer(t)
	return contactsv1connect.NewContactsServiceClient(ts.Client(), ts.URL)
}

func TestListContacts_Unauthenticated(t *testing.T) {
	client := contactsClient(t)
	_, err := client.ListContacts(
		context.Background(),
		connect.NewRequest(&contactsv1.ListContactsRequest{}),
	)
	require.Error(t, err)
}

func TestListContacts_Empty(t *testing.T) {
	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.ListContactsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.ListContacts(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Contacts)
}

func TestCreateContact_Success(t *testing.T) {
	ctx := context.Background()
	otherID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	otherEmail := "other-contact@example.com"
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, testUserID, "user@example.com"))
	require.NoError(t, testApp.appUsersRepo.Upsert(ctx, otherID, otherEmail))

	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.CreateContactRequest{
		Email:       otherEmail,
		DisplayName: "Other User",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.CreateContact(context.Background(), req)
	require.NoError(t, err)
}

func TestCreateContact_NotFound(t *testing.T) {
	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.CreateContactRequest{
		Email:       "nonexistent@nowhere.example",
		DisplayName: "Nobody",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.CreateContact(context.Background(), req)
	require.Error(t, err)
}

func TestAcceptContact_InvalidUUID(t *testing.T) {
	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.AcceptContactRequest{
		Id:          "not-a-uuid",
		DisplayName: "Test",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.AcceptContact(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestAcceptContact_Success(t *testing.T) {
	contactID := insertPendingContact(t)

	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.AcceptContactRequest{
		Id:          contactID,
		DisplayName: "Accepted Friend",
	})
	setCookieOnRequest(req, accessToken)
	_, err := client.AcceptContact(context.Background(), req)
	require.NoError(t, err)
}

func TestDeclineContact_InvalidUUID(t *testing.T) {
	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.DeclineContactRequest{Id: "not-a-uuid"})
	setCookieOnRequest(req, accessToken)
	_, err := client.DeclineContact(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestDeclineContact_Success(t *testing.T) {
	contactID := insertPendingContact(t)

	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.DeclineContactRequest{Id: contactID})
	setCookieOnRequest(req, accessToken)
	_, err := client.DeclineContact(context.Background(), req)
	require.NoError(t, err)
}

func TestDeleteContact_InvalidUUID(t *testing.T) {
	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.DeleteContactRequest{Id: "not-a-uuid"})
	setCookieOnRequest(req, accessToken)
	_, err := client.DeleteContact(context.Background(), req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestDeleteContact_Success(t *testing.T) {
	contactID := insertPendingContact(t)
	ctx := context.Background()
	id := mustParseUUID(t, contactID)
	require.NoError(t, testApp.contacts.Accept(ctx, id, testUserID, "Sender"))

	accepted, err := testApp.contacts.List(ctx, testUserID)
	require.NoError(t, err)
	require.NotEmpty(t, accepted)
	acceptedID := accepted[0].ID.String()

	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.DeleteContactRequest{Id: acceptedID})
	setCookieOnRequest(req, accessToken)
	_, err = client.DeleteContact(context.Background(), req)
	require.NoError(t, err)
}

func TestListContacts_WithIncoming(t *testing.T) {
	_ = insertPendingContact(t)

	client := contactsClient(t)
	req := connect.NewRequest(&contactsv1.ListContactsRequest{})
	setCookieOnRequest(req, accessToken)
	resp, err := client.ListContacts(context.Background(), req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Incoming)
}
