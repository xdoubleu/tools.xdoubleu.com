// Package connecttools contains helpers shared by apps' ConnectRPC handlers.
package connecttools

import (
	"errors"
	"net/http"

	"connectrpc.com/connect/v2"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
)

// ErrorCode maps an app's own domain error to a Connect code.
type ErrorCode struct {
	target  error
	code    connect.Code
	message string
}

// Code maps errors matching target (errors.Is) to code.
func Code(target error, code connect.Code) ErrorCode {
	return ErrorCode{target: target, code: code, message: ""}
}

// CodeWithMessage is Code with a fixed client-facing message.
func CodeWithMessage(target error, code connect.Code, message string) ErrorCode {
	return ErrorCode{target: target, code: code, message: message}
}

// MapError translates a service-layer error into the matching ConnectRPC
// error. A *connect.Error passes through; app codes are checked before the
// shared database and HTTPError mappings; anything else is CodeInternal.
func MapError(err error, app ...ErrorCode) error {
	if err == nil {
		return nil
	}
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return connectErr
	}
	for _, c := range app {
		if errors.Is(err, c.target) {
			msg := c.message
			if msg == "" {
				msg = err.Error()
			}
			return connect.NewError(c.code, msg).WithCause(err)
		}
	}
	return connect.NewError(sharedCode(err), err.Error()).WithCause(err)
}

func sharedCode(err error) connect.Code {
	switch {
	case errors.Is(err, database.ErrResourceNotFound):
		return connect.CodeNotFound
	case errors.Is(err, database.ErrResourceConflict):
		return connect.CodeAlreadyExists
	}

	var httpErr *iapp.HTTPError
	if !errors.As(err, &httpErr) {
		return connect.CodeInternal
	}
	switch httpErr.Status {
	case http.StatusBadRequest:
		return connect.CodeInvalidArgument
	case http.StatusNotFound:
		return connect.CodeNotFound
	case http.StatusConflict:
		return connect.CodeAlreadyExists
	case http.StatusForbidden:
		return connect.CodePermissionDenied
	default:
		return connect.CodeInternal
	}
}
