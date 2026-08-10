package connecttools_test

import (
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/database"
)

func TestMapError_Nil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, connecttools.MapError(nil))
}

func TestMapError_ResourceNotFound(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(database.ErrResourceNotFound)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestMapError_ResourceConflict(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(database.ErrResourceConflict)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
}

func TestMapError_HTTPBadRequest(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		&iapp.HTTPError{Status: http.StatusBadRequest, Message: "bad"},
	)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestMapError_HTTPNotFound(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		&iapp.HTTPError{Status: http.StatusNotFound, Message: "not found"},
	)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestMapError_HTTPConflict(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		&iapp.HTTPError{Status: http.StatusConflict, Message: "conflict"},
	)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
}

func TestMapError_HTTPForbidden(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		&iapp.HTTPError{Status: http.StatusForbidden, Message: "forbidden"},
	)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

func TestMapError_HTTPOther(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		&iapp.HTTPError{Status: http.StatusInternalServerError, Message: "oops"},
	)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

func TestMapError_GenericError(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(errors.New("some error"))
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}
