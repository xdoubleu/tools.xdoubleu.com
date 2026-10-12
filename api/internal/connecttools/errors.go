// Package connecttools contains helpers shared by apps' ConnectRPC handlers.
package connecttools

import (
	"errors"
	"net/http"

	"connectrpc.com/connect/v2"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
)

// MapError translates a service-layer error into the matching ConnectRPC error.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, database.ErrResourceNotFound) {
		return connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
	}
	if errors.Is(err, database.ErrResourceConflict) {
		return connect.NewError(connect.CodeAlreadyExists, err.Error()).WithCause(err)
	}

	var httpErr *iapp.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.Status {
		case http.StatusBadRequest:
			return connect.NewError(connect.CodeInvalidArgument, err.Error()).WithCause(err)
		case http.StatusNotFound:
			return connect.NewError(connect.CodeNotFound, err.Error()).WithCause(err)
		case http.StatusConflict:
			return connect.NewError(connect.CodeAlreadyExists, err.Error()).WithCause(err)
		case http.StatusForbidden:
			return connect.NewError(connect.CodePermissionDenied, err.Error()).WithCause(err)
		default:
			return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
		}
	}

	return connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
}
