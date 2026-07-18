package reading

import (
	"context"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/services"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// koboDeviceForLogging resolves the authenticated user, parses the device ID,
// and confirms the device belongs to that user. It returns the verified device
// ID or a Connect error suitable for returning directly.
func (h *booksConnectHandler) koboDeviceForLogging(
	ctx context.Context,
	rawID string,
) (uuid.UUID, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return uuid.Nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	deviceID, err := uuid.Parse(rawID)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid device ID"),
		)
	}
	if _, err = h.app.Services.Kobo.GetKoboDevice(
		ctx, user.ID, deviceID,
	); err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return uuid.Nil, connect.NewError(
				connect.CodeNotFound,
				errors.New("device not found"),
			)
		}
		return uuid.Nil, connect.NewError(connect.CodeInternal, err)
	}
	return deviceID, nil
}

func (h *booksConnectHandler) SetKoboDeviceLogging(
	ctx context.Context,
	req *connect.Request[readingv1.SetKoboDeviceLoggingRequest],
) (*connect.Response[readingv1.SetKoboDeviceLoggingResponse], error) {
	deviceID, err := h.koboDeviceForLogging(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	h.app.Services.KoboLog.SetEnabled(deviceID.String(), req.Msg.Enabled)
	return connect.NewResponse(&readingv1.SetKoboDeviceLoggingResponse{}), nil
}

func (h *booksConnectHandler) GetKoboDeviceLogs(
	ctx context.Context,
	req *connect.Request[readingv1.GetKoboDeviceLogsRequest],
) (*connect.Response[readingv1.GetKoboDeviceLogsResponse], error) {
	deviceID, err := h.koboDeviceForLogging(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	entries := h.app.Services.KoboLog.List(deviceID.String())
	resp := &readingv1.GetKoboDeviceLogsResponse{
		Entries: make([]*readingv1.KoboLogEntry, len(entries)),
	}
	for i, e := range entries {
		resp.Entries[i] = koboLogEntryProto(e)
	}
	return connect.NewResponse(resp), nil
}

func (h *booksConnectHandler) ClearKoboDeviceLogs(
	ctx context.Context,
	req *connect.Request[readingv1.ClearKoboDeviceLogsRequest],
) (*connect.Response[readingv1.ClearKoboDeviceLogsResponse], error) {
	deviceID, err := h.koboDeviceForLogging(ctx, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	h.app.Services.KoboLog.Clear(deviceID.String())
	return connect.NewResponse(&readingv1.ClearKoboDeviceLogsResponse{}), nil
}

// koboLogEntryProto converts a captured entry to its proto form. Captured
// bodies come from arbitrary Kobo/upstream traffic (including binary/gzip
// responses proxied verbatim) and the 64KB capture cap can slice a body
// mid-rune, so they are not guaranteed valid UTF-8. proto3 string fields must
// be valid UTF-8 or marshaling fails, so sanitize here — the single point
// every captured entry passes through on its way into the response.
func koboLogEntryProto(e services.KoboLogEntry) *readingv1.KoboLogEntry {
	return &readingv1.KoboLogEntry{
		Time:         e.Time.Format(time.RFC3339),
		Method:       e.Method,
		Path:         e.Path,
		Query:        e.Query,
		RequestBody:  strings.ToValidUTF8(e.RequestBody, "�"),
		Status:       int32(e.Status), //nolint:gosec // HTTP status fits int32
		ResponseBody: strings.ToValidUTF8(e.ResponseBody, "�"),
	}
}
