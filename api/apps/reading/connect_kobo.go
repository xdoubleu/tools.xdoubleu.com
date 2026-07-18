package reading

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/models"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) EnableKoboSync(
	ctx context.Context,
	req *connect.Request[readingv1.EnableKoboSyncRequest],
) (*connect.Response[readingv1.EnableKoboSyncResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	bookID, err := uuid.Parse(req.Msg.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid book ID"),
		)
	}
	if err = h.app.Services.Books.EnableKoboSync(ctx, user.ID, bookID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	statusResult, err := h.app.Services.Books.GetKEPUBStatus(ctx, user.ID, bookID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	kepubStatus, convErr := h.maybeStartKEPUBConversion(
		ctx, user.ID, bookID, statusResult, true,
	)
	if convErr != nil {
		return nil, connect.NewError(connect.CodeInternal, convErr)
	}
	return connect.NewResponse(&readingv1.EnableKoboSyncResponse{
		KepubStatus: kepubStatus,
	}), nil
}

func (h *booksConnectHandler) RegisterKoboDevice(
	ctx context.Context,
	req *connect.Request[readingv1.RegisterKoboDeviceRequest],
) (*connect.Response[readingv1.RegisterKoboDeviceResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	device, rawToken, err := h.app.Services.Kobo.RegisterKoboDevice(
		ctx, user.ID, req.Msg.Name, req.Msg.Serial,
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.RegisterKoboDeviceResponse{
		Device:   koboDeviceProto(device),
		RawToken: rawToken,
	}), nil
}

func (h *booksConnectHandler) ListKoboDevices(
	ctx context.Context,
	_ *connect.Request[readingv1.ListKoboDevicesRequest],
) (*connect.Response[readingv1.ListKoboDevicesResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	devices, err := h.app.Services.Kobo.ListKoboDevices(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &readingv1.ListKoboDevicesResponse{
		Devices: make([]*readingv1.KoboDevice, len(devices)),
	}
	for i, d := range devices {
		// Debug-logging state lives in memory, not the DB row.
		d.LoggingEnabled = h.app.Services.KoboLog.IsEnabled(d.ID)
		resp.Devices[i] = koboDeviceProto(d)
	}
	return connect.NewResponse(resp), nil
}

func (h *booksConnectHandler) DisconnectKoboDevice(
	ctx context.Context,
	req *connect.Request[readingv1.DisconnectKoboDeviceRequest],
) (*connect.Response[readingv1.DisconnectKoboDeviceResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	deviceID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid device ID"),
		)
	}
	err = h.app.Services.Kobo.DisconnectKoboDevice(ctx, user.ID, deviceID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return nil, connect.NewError(
				connect.CodeNotFound,
				errors.New("device not found"),
			)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&readingv1.DisconnectKoboDeviceResponse{}), nil
}

func koboDeviceProto(d models.KoboDevice) *readingv1.KoboDevice {
	lastSeen := ""
	if d.LastSeenAt != nil {
		lastSeen = d.LastSeenAt.Format(time.RFC3339)
	}
	return &readingv1.KoboDevice{
		Id:             d.ID,
		Name:           d.Name,
		Serial:         d.Serial,
		CreatedAt:      d.CreatedAt.Format(time.RFC3339),
		LastSeenAt:     lastSeen,
		LoggingEnabled: d.LoggingEnabled,
	}
}
