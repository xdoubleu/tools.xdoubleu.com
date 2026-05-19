package contacts_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/contacts"
	"tools.xdoubleu.com/internal/mocks"
)

// TestNotFoundError_Error verifies that the error returned when a user is not
// found by email has a non-empty message.
func TestNotFoundError_Error(t *testing.T) {
	// The mock auth service only knows one user (email "user@example.com").
	// Searching for a different email triggers the notFoundError path.
	svc := contacts.New(nil, mocks.NewMockedAuthService("test-user"))
	err := svc.AddByEmail(
		context.Background(),
		"test-user",
		"nonexistent@nowhere.example",
		"Nobody",
	)
	require.Error(t, err)
	assert.NotEmpty(t, err.Error())
}

// TestAddByEmail_UserNotFound confirms AddByEmail returns an error when the
// email does not match any user known to the auth service.
func TestAddByEmail_UserNotFound(t *testing.T) {
	svc := contacts.New(nil, mocks.NewMockedAuthService("test-user"))

	err := svc.AddByEmail(
		context.Background(),
		"test-user",
		"nonexistent@nowhere.example",
		"Nobody",
	)
	require.Error(t, err)
}

// TestNotFoundError_HTTPStatus verifies that the error returned by AddByEmail
// exposes HTTPStatus() == 404 via the httptools-compatible interface.
func TestNotFoundError_HTTPStatus(t *testing.T) {
	type httpStatuser interface {
		HTTPStatus() int
	}
	svc := contacts.New(nil, mocks.NewMockedAuthService("test-user"))
	err := svc.AddByEmail(
		context.Background(),
		"test-user",
		"nonexistent@nowhere.example",
		"Nobody",
	)
	require.Error(t, err)
	hs, ok := err.(httpStatuser)
	require.True(t, ok, "error should implement HTTPStatus()")
	assert.Equal(t, 404, hs.HTTPStatus())
}
