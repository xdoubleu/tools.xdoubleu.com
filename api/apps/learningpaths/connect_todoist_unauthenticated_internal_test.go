package learningpaths

// Package-internal test: getUser(ctx) returns nil (and each handler returns
// CodeUnauthenticated) before it ever touches h.app, so a nil app is safe
// here — mirrors connect_unauthenticated_internal_test.go for TodoistService.

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"

	learningpathsv1 "tools.xdoubleu.com/gen/learningpaths/v1"
)

func TestConnectTodoist_Unauthenticated(t *testing.T) {
	h := &todoistConnectHandler{app: nil}
	_, err := h.ConnectTodoist(
		context.Background(),
		connect.NewRequest(&learningpathsv1.ConnectTodoistRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestDisconnectTodoist_Unauthenticated(t *testing.T) {
	h := &todoistConnectHandler{app: nil}
	_, err := h.DisconnectTodoist(
		context.Background(),
		connect.NewRequest(&learningpathsv1.DisconnectTodoistRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestGetTodoistConnectionStatus_Unauthenticated(t *testing.T) {
	h := &todoistConnectHandler{app: nil}
	_, err := h.GetTodoistConnectionStatus(
		context.Background(),
		connect.NewRequest(&learningpathsv1.GetTodoistConnectionStatusRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestSendItemToTodoist_Unauthenticated(t *testing.T) {
	h := &todoistConnectHandler{app: nil}
	_, err := h.SendItemToTodoist(
		context.Background(),
		connect.NewRequest(&learningpathsv1.SendItemToTodoistRequest{}),
	)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
