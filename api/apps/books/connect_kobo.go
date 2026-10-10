package books

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (h *booksConnectHandler) EnableKoboSync(
	ctx context.Context,
	req *connect.Request[booksv1.EnableKoboSyncRequest],
) (*connect.Response[booksv1.EnableKoboSyncResponse], error) {
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
	return connect.NewResponse(&booksv1.EnableKoboSyncResponse{
		KepubStatus: kepubStatus,
	}), nil
}

func (h *booksConnectHandler) RegisterKoboDevice(
	ctx context.Context,
	req *connect.Request[booksv1.RegisterKoboDeviceRequest],
) (*connect.Response[booksv1.RegisterKoboDeviceResponse], error) {
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
	return connect.NewResponse(&booksv1.RegisterKoboDeviceResponse{
		Device:   koboDeviceProto(device),
		RawToken: rawToken,
	}), nil
}

func (h *booksConnectHandler) ListKoboDevices(
	ctx context.Context,
	_ *connect.Request[booksv1.ListKoboDevicesRequest],
) (*connect.Response[booksv1.ListKoboDevicesResponse], error) {
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
	resp := &booksv1.ListKoboDevicesResponse{
		Devices: make([]*booksv1.KoboDevice, len(devices)),
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
	req *connect.Request[booksv1.DisconnectKoboDeviceRequest],
) (*connect.Response[booksv1.DisconnectKoboDeviceResponse], error) {
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
	return connect.NewResponse(&booksv1.DisconnectKoboDeviceResponse{}), nil
}

func koboDeviceProto(d models.KoboDevice) *booksv1.KoboDevice {
	lastSeen := ""
	if d.LastSeenAt != nil {
		lastSeen = d.LastSeenAt.Format(time.RFC3339)
	}
	return &booksv1.KoboDevice{
		Id:             d.ID,
		Name:           d.Name,
		Serial:         d.Serial,
		CreatedAt:      d.CreatedAt.Format(time.RFC3339),
		LastSeenAt:     lastSeen,
		LoggingEnabled: d.LoggingEnabled,
	}
}

func (h *booksConnectHandler) ListKoboStoreBooks(
	ctx context.Context,
	_ *connect.Request[booksv1.ListKoboStoreBooksRequest],
) (*connect.Response[booksv1.ListKoboStoreBooksResponse], error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	books, err := h.app.Services.Books.ListKoboStoreBooks(ctx, user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &booksv1.ListKoboStoreBooksResponse{
		Books: make([]*booksv1.KoboStoreBook, len(books)),
	}
	for i, b := range books {
		resp.Books[i] = koboStoreBookProto(b)
	}
	return connect.NewResponse(resp), nil
}

func koboStoreBookProto(b models.KoboStoreBookStatus) *booksv1.KoboStoreBook {
	out := &booksv1.KoboStoreBook{
		EntitlementId:  b.EntitlementID,
		Title:          b.Title,
		Authors:        b.Authors,
		Owned:          b.Owned != nil && *b.Owned,
		LastOutcome:    string(b.LastOutcome),
		LastReadAt:     formatOptionalTime(b.LastReadAt),
		LastMirroredAt: formatOptionalTime(b.LastMirroredAt),
		UpdatedAt:      b.UpdatedAt.Format(time.RFC3339),
	}
	if b.ISBN13 != nil {
		out.Isbn13 = *b.ISBN13
	}
	if b.LastPercent != nil {
		p := int32(*b.LastPercent) //nolint:gosec // a 0-100 percent
		out.LastPercent = &p
	}
	if b.Match != nil {
		out.LibraryBookId = b.Match.BookID.String()
		if b.Match.Book != nil {
			out.LibraryTitle = b.Match.Book.Title
		}
	}
	return out
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
