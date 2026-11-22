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

func (h *Handler) EnableKoboSync(
	ctx context.Context,
	req *booksv1.EnableKoboSyncRequest,
) (*booksv1.EnableKoboSyncResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	bookID, err := uuid.Parse(req.BookId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid book ID",
		)
	}
	kepubStatus, err := h.services.Library.EnableKoboSync(ctx, user.ID, bookID)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.EnableKoboSyncResponse{
		KepubStatus: kepubStatus,
	}, nil
}

func (h *Handler) RegisterKoboDevice(
	ctx context.Context,
	req *booksv1.RegisterKoboDeviceRequest,
) (*booksv1.RegisterKoboDeviceResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	device, rawToken, err := h.services.Kobo.RegisterKoboDevice(
		ctx, user.ID, req.Name, req.Serial,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.RegisterKoboDeviceResponse{
		Device:   protomap.KoboDeviceProto(device),
		RawToken: rawToken,
	}, nil
}

func (h *Handler) ListKoboDevices(
	ctx context.Context,
	_ *booksv1.ListKoboDevicesRequest,
) (*booksv1.ListKoboDevicesResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	devices, err := h.services.Library.KoboDevices(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.KoboDevicesResponse(devices), nil
}

func (h *Handler) DisconnectKoboDevice(
	ctx context.Context,
	req *booksv1.DisconnectKoboDeviceRequest,
) (*booksv1.DisconnectKoboDeviceResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	deviceID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid device ID",
		)
	}
	err = h.services.Kobo.DisconnectKoboDevice(ctx, user.ID, deviceID)
	if err != nil {
		return nil, mapError(err)
	}
	return &booksv1.DisconnectKoboDeviceResponse{}, nil
}

func (h *Handler) ListKoboStoreBooks(
	ctx context.Context,
	_ *booksv1.ListKoboStoreBooksRequest,
) (*booksv1.ListKoboStoreBooksResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	books, err := h.services.Books.ListKoboStoreBooks(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.KoboStoreBooksResponse(books), nil
}
