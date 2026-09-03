package main

import (
	"context"
	"time"

	"connectrpc.com/connect"

	familyv1 "tools.xdoubleu.com/gen/family/v1"
	"tools.xdoubleu.com/gen/family/v1/familyv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/family"
	"tools.xdoubleu.com/internal/models"
)

type familyConnectHandler struct {
	app *Application
}

var _ familyv1connect.FamilyServiceHandler = (*familyConnectHandler)(nil)

func (h *familyConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	return u.ID
}

// emailsByUserID resolves user IDs to their email addresses (family members
// and invites are stored by user ID, but displayed by email).
func (h *familyConnectHandler) emailsByUserID(
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

func (h *familyConnectHandler) GetFamily(
	ctx context.Context,
	_ *connect.Request[familyv1.GetFamilyRequest],
) (*connect.Response[familyv1.GetFamilyResponse], error) {
	userID := h.userID(ctx)

	membership, err := h.app.family.GetMembership(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	invite, hasInvite, err := h.app.family.GetIncomingInvite(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	emails, err := h.emailsByUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	members := make([]*familyv1.FamilyMember, len(membership.Members))
	for i, memberID := range membership.Members {
		members[i] = &familyv1.FamilyMember{
			UserId:      memberID,
			Email:       emails[memberID],
			DisplayName: membership.DisplayNames[memberID],
		}
	}

	resp := &familyv1.GetFamilyResponse{
		Members:         members,
		IncomingInvite:  nil,
		SelfDisplayName: membership.SelfDisplayName,
	}
	if hasInvite {
		resp.IncomingInvite = &familyv1.FamilyInvite{
			Id:         invite.ID.String(),
			FromUserId: invite.FromUserID,
			FromEmail:  emails[invite.FromUserID],
			CreatedAt:  invite.CreatedAt.Format(time.RFC3339),
		}
	}

	return connect.NewResponse(resp), nil
}

func (h *familyConnectHandler) InviteToFamily(
	ctx context.Context,
	req *connect.Request[familyv1.InviteToFamilyRequest],
) (*connect.Response[familyv1.InviteToFamilyResponse], error) {
	userID := h.userID(ctx)

	if err := h.app.family.InviteByEmail(ctx, userID, req.Msg.Email); err != nil {
		if family.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&familyv1.InviteToFamilyResponse{}), nil
}

func (h *familyConnectHandler) AcceptFamilyInvite(
	ctx context.Context,
	_ *connect.Request[familyv1.AcceptFamilyInviteRequest],
) (*connect.Response[familyv1.AcceptFamilyInviteResponse], error) {
	userID := h.userID(ctx)

	if err := h.app.family.Accept(ctx, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&familyv1.AcceptFamilyInviteResponse{}), nil
}

func (h *familyConnectHandler) DeclineFamilyInvite(
	ctx context.Context,
	_ *connect.Request[familyv1.DeclineFamilyInviteRequest],
) (*connect.Response[familyv1.DeclineFamilyInviteResponse], error) {
	userID := h.userID(ctx)

	if err := h.app.family.Decline(ctx, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&familyv1.DeclineFamilyInviteResponse{}), nil
}

func (h *familyConnectHandler) SetFamilyDisplayName(
	ctx context.Context,
	req *connect.Request[familyv1.SetFamilyDisplayNameRequest],
) (*connect.Response[familyv1.SetFamilyDisplayNameResponse], error) {
	userID := h.userID(ctx)

	if err := h.app.family.SetDisplayName(
		ctx, userID, req.Msg.DisplayName,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&familyv1.SetFamilyDisplayNameResponse{}), nil
}

func (h *familyConnectHandler) LeaveFamily(
	ctx context.Context,
	_ *connect.Request[familyv1.LeaveFamilyRequest],
) (*connect.Response[familyv1.LeaveFamilyResponse], error) {
	userID := h.userID(ctx)

	if err := h.app.family.Leave(ctx, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&familyv1.LeaveFamilyResponse{}), nil
}
