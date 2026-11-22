package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/protomap"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// koboDeviceForLogging resolves the authenticated user, parses the device ID,
// and confirms the device belongs to that user. It returns the verified device
// ID or a Connect error suitable for returning directly.
func (h *Handler) koboDeviceForLogging(
	ctx context.Context,
	rawID string,
) (uuid.UUID, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return uuid.Nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	deviceID, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid device ID",
		)
	}
	if _, err = h.services.Kobo.GetKoboDevice(
		ctx, user.ID, deviceID,
	); err != nil {
		return uuid.Nil, mapError(err)
	}
	return deviceID, nil
}

func (h *Handler) SetKoboDeviceLogging(
	ctx context.Context,
	req *booksv1.SetKoboDeviceLoggingRequest,
) (*booksv1.SetKoboDeviceLoggingResponse, error) {
	deviceID, err := h.koboDeviceForLogging(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	h.services.KoboLog.SetEnabled(deviceID.String(), req.Enabled)
	return &booksv1.SetKoboDeviceLoggingResponse{}, nil
}

func (h *Handler) GetKoboDeviceLogs(
	ctx context.Context,
	req *booksv1.GetKoboDeviceLogsRequest,
) (*booksv1.GetKoboDeviceLogsResponse, error) {
	user := caller(ctx)
	if user == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, "unauthorized")
	}
	deviceID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "invalid device ID")
	}
	entries, err := h.services.Library.KoboDeviceLogs(ctx, user.ID, deviceID)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.KoboLogsResponse(entries), nil
}

func (h *Handler) ClearKoboDeviceLogs(
	ctx context.Context,
	req *booksv1.ClearKoboDeviceLogsRequest,
) (*booksv1.ClearKoboDeviceLogsResponse, error) {
	deviceID, err := h.koboDeviceForLogging(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	h.services.KoboLog.Clear(deviceID.String())
	return &booksv1.ClearKoboDeviceLogsResponse{}, nil
}
