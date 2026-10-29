package account

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	familyv1 "tools.xdoubleu.com/gen/family/v1"
	"tools.xdoubleu.com/gen/family/v1/familyv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/family"
	"tools.xdoubleu.com/internal/models"
)

type familyConnectHandler struct {
	app *Deps
}

var _ familyv1connect.FamilyServiceHandler = (*familyConnectHandler)(nil)

func (h *familyConnectHandler) userID(ctx context.Context) string {
	u := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	return u.ID
}

// emailsByUserID resolves user IDs to emails for display.
func (h *familyConnectHandler) emailsByUserID(
	ctx context.Context,
) (map[string]string, error) {
	users, err := h.app.Auth.GetAllUsers(ctx)
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
	_ *familyv1.GetFamilyRequest,
) (*familyv1.GetFamilyResponse, error) {
	userID := h.userID(ctx)

	membership, err := h.app.Family.GetMembership(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	invite, hasInvite, err := h.app.Family.GetIncomingInvite(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	emails, err := h.emailsByUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
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

	return resp, nil
}

func (h *familyConnectHandler) InviteToFamily(
	ctx context.Context,
	req *familyv1.InviteToFamilyRequest,
) (*familyv1.InviteToFamilyResponse, error) {
	userID := h.userID(ctx)

	if err := h.app.Family.InviteByEmail(ctx, userID, req.Email); err != nil {
		if family.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
		}
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &familyv1.InviteToFamilyResponse{}, nil
}

func (h *familyConnectHandler) AcceptFamilyInvite(
	ctx context.Context,
	req *familyv1.AcceptFamilyInviteRequest,
) (*familyv1.AcceptFamilyInviteResponse, error) {
	userID := h.userID(ctx)

	inviteID, err := uuid.Parse(req.InviteId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, "invalid invite ID",
		)
	}

	if err = h.app.Family.Accept(ctx, userID, inviteID); err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return nil, connect.NewError(
				connect.CodeNotFound, "invite is no longer pending",
			)
		}
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &familyv1.AcceptFamilyInviteResponse{}, nil
}

func (h *familyConnectHandler) DeclineFamilyInvite(
	ctx context.Context,
	_ *familyv1.DeclineFamilyInviteRequest,
) (*familyv1.DeclineFamilyInviteResponse, error) {
	userID := h.userID(ctx)

	if err := h.app.Family.Decline(ctx, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &familyv1.DeclineFamilyInviteResponse{}, nil
}

func (h *familyConnectHandler) SetFamilyDisplayName(
	ctx context.Context,
	req *familyv1.SetFamilyDisplayNameRequest,
) (*familyv1.SetFamilyDisplayNameResponse, error) {
	userID := h.userID(ctx)

	if err := h.app.Family.SetDisplayName(
		ctx, userID, req.DisplayName,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &familyv1.SetFamilyDisplayNameResponse{}, nil
}

func (h *familyConnectHandler) LeaveFamily(
	ctx context.Context,
	_ *familyv1.LeaveFamilyRequest,
) (*familyv1.LeaveFamilyResponse, error) {
	userID := h.userID(ctx)

	if err := h.app.Family.Leave(ctx, userID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}

	return &familyv1.LeaveFamilyResponse{}, nil
}
