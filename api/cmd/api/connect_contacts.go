package main

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"

	contactsv1 "tools.xdoubleu.com/gen/contacts/v1"
	"tools.xdoubleu.com/gen/contacts/v1/contactsv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

type contactsConnectHandler struct {
	app *Application
}

var _ contactsv1connect.ContactsServiceHandler = (*contactsConnectHandler)(nil)

func (h *contactsConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	return u.ID
}

func protoContact(c models.Contact, emails map[string]string) *contactsv1.Contact {
	return &contactsv1.Contact{
		Id:            c.ID.String(),
		OwnerUserId:   c.OwnerUserID,
		ContactUserId: c.ContactUserID,
		DisplayName:   c.DisplayName,
		Status:        c.Status,
		CreatedAt:     c.CreatedAt.Format(time.RFC3339),
		OwnerEmail:    emails[c.OwnerUserID],
		ContactEmail:  emails[c.ContactUserID],
	}
}

func protoContactSlice(
	cs []models.Contact,
	emails map[string]string,
) []*contactsv1.Contact {
	out := make([]*contactsv1.Contact, len(cs))
	for i, c := range cs {
		out[i] = protoContact(c, emails)
	}
	return out
}

func (h *contactsConnectHandler) ListContacts(
	ctx context.Context,
	_ *connect.Request[contactsv1.ListContactsRequest],
) (*connect.Response[contactsv1.ListContactsResponse], error) {
	userID := h.userID(ctx)

	contacts, err := h.app.contacts.List(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	pending, err := h.app.contacts.ListPending(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	incoming, err := h.app.contacts.ListIncoming(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	emails, err := h.emailsByUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&contactsv1.ListContactsResponse{
		Contacts: protoContactSlice(contacts, emails),
		Pending:  protoContactSlice(pending, emails),
		Incoming: protoContactSlice(incoming, emails),
	}), nil
}

// emailsByUserID resolves user IDs to their email addresses so contacts,
// which are stored by user ID, can be displayed by email.
func (h *contactsConnectHandler) emailsByUserID(
	ctx context.Context,
) (map[string]string, error) {
	users, err := h.app.auth.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}

	emails := make(map[string]string, len(users))
	for _, u := range users {
		emails[u.ID] = u.Email
	}
	return emails, nil
}

func (h *contactsConnectHandler) CreateContact(
	ctx context.Context,
	req *connect.Request[contactsv1.CreateContactRequest],
) (*connect.Response[contactsv1.CreateContactResponse], error) {
	userID := h.userID(ctx)

	if err := h.app.contacts.AddByEmail(
		ctx, userID, req.Msg.Email, req.Msg.DisplayName,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&contactsv1.CreateContactResponse{}), nil
}

func (h *contactsConnectHandler) AcceptContact(
	ctx context.Context,
	req *connect.Request[contactsv1.AcceptContactRequest],
) (*connect.Response[contactsv1.AcceptContactResponse], error) {
	userID := h.userID(ctx)

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid contact id"),
		)
	}

	if err = h.app.contacts.Accept(ctx, id, userID, req.Msg.DisplayName); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&contactsv1.AcceptContactResponse{}), nil
}

func (h *contactsConnectHandler) DeclineContact(
	ctx context.Context,
	req *connect.Request[contactsv1.DeclineContactRequest],
) (*connect.Response[contactsv1.DeclineContactResponse], error) {
	userID := h.userID(ctx)

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid contact id"),
		)
	}

	if err = h.app.contacts.Decline(ctx, id, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&contactsv1.DeclineContactResponse{}), nil
}

func (h *contactsConnectHandler) UpdateContact(
	ctx context.Context,
	req *connect.Request[contactsv1.UpdateContactRequest],
) (*connect.Response[contactsv1.UpdateContactResponse], error) {
	userID := h.userID(ctx)

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid contact id"),
		)
	}

	if err = h.app.contacts.Update(ctx, id, userID, req.Msg.DisplayName); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&contactsv1.UpdateContactResponse{}), nil
}

func (h *contactsConnectHandler) DeleteContact(
	ctx context.Context,
	req *connect.Request[contactsv1.DeleteContactRequest],
) (*connect.Response[contactsv1.DeleteContactResponse], error) {
	userID := h.userID(ctx)

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid contact id"),
		)
	}

	if err = h.app.contacts.Delete(ctx, id, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&contactsv1.DeleteContactResponse{}), nil
}
