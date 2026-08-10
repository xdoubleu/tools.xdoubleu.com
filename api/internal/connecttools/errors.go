// Package connecttools contains helpers shared by apps' ConnectRPC handlers.
package connecttools

import (
	"errors"
	"net/http"

	"connectrpc.com/connect"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
)

// MapError translates a service-layer error into the matching ConnectRPC
// error, so every app's ConnectRPC handlers report consistent codes for
// [database.ErrResourceNotFound]/[database.ErrResourceConflict] and any
// [iapp.HTTPError].
func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, database.ErrResourceNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	if errors.Is(err, database.ErrResourceConflict) {
		return connect.NewError(connect.CodeAlreadyExists, err)
	}

	var httpErr *iapp.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusBadRequest:
			return connect.NewError(connect.CodeInvalidArgument, err)
		case http.StatusNotFound:
			return connect.NewError(connect.CodeNotFound, err)
		case http.StatusConflict:
			return connect.NewError(connect.CodeAlreadyExists, err)
		case http.StatusForbidden:
			return connect.NewError(connect.CodePermissionDenied, err)
		default:
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewError(connect.CodeInternal, err)
}
