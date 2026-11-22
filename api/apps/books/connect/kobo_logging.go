package connect

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/kobo"
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
	return KoboLogsResponse(entries), nil
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

// koboLogEntryProto converts a captured entry to its proto form. Captured
// bodies come from arbitrary Kobo/upstream traffic (including binary/gzip
// responses proxied verbatim) and the 64KB capture cap can slice a body
// mid-rune, so they are not guaranteed valid UTF-8. proto3 string fields must
// be valid UTF-8 or marshaling fails, so sanitize here — the single point
// every captured entry passes through on its way into the response.
func koboLogEntryProto(e kobo.LogEntry) *booksv1.KoboLogEntry {
	return &booksv1.KoboLogEntry{
		Time:         e.Time.Format(time.RFC3339),
		Method:       e.Method,
		Path:         e.Path,
		Query:        e.Query,
		RequestBody:  strings.ToValidUTF8(e.RequestBody, "�"),
		Status:       int32(e.Status), //nolint:gosec // HTTP status fits int32
		ResponseBody: strings.ToValidUTF8(e.ResponseBody, "�"),
		Notes:        e.Notes,
		// Header values are proxied from the device and upstream verbatim.
		RequestHeaders:  validUTF8Map(e.RequestHeaders),
		ResponseHeaders: validUTF8Map(e.ResponseHeaders),
		UpstreamHeaders: validUTF8Map(e.UpstreamHeaders),
	}
}

func validUTF8Map(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = strings.ToValidUTF8(v, "�")
	}
	return out
}
