package watchparty_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
)

const presenterID = "presenter-user-id-for-testing"

type joinRoomForm struct {
	RoomCode string `schema:"roomCode"`
}

func TestRootShowsLobbyWhenNotInRoom(t *testing.T) {
	_, routes := newTestApp()

	tReq := test.CreateRequestTester(
		routes,
		http.MethodGet,
		fmt.Sprintf("/%s/", "watchparty"),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestRootShowsPresenterView(t *testing.T) {
	app, routes := newTestApp()

	app.Services.Room.CreateRoom(context.Background(), userID)

	tReq := test.CreateRequestTester(
		routes,
		http.MethodGet,
		fmt.Sprintf("/%s/", "watchparty"),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestRootShowsViewerView(t *testing.T) {
	app, routes := newTestApp()

	roomCode := app.Services.Room.CreateRoom(context.Background(), presenterID)
	app.Services.Room.JoinViewer(context.Background(), roomCode, userID)

	tReq := test.CreateRequestTester(
		routes,
		http.MethodGet,
		fmt.Sprintf("/%s/", "watchparty"),
	)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestCreateRoom(t *testing.T) {
	_, routes := newTestApp()

	tReq := test.CreateRequestTester(
		routes,
		http.MethodPost,
		fmt.Sprintf("/%s/api/rooms/create", "watchparty"),
	)
	tReq.SetFollowRedirect(false)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestCreateRoomWhenAlreadyPresenter(t *testing.T) {
	app, routes := newTestApp()

	app.Services.Room.CreateRoom(context.Background(), userID)

	tReq := test.CreateRequestTester(
		routes,
		http.MethodPost,
		fmt.Sprintf("/%s/api/rooms/create", "watchparty"),
	)
	tReq.SetFollowRedirect(false)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestJoinRoomWithEmptyCode(t *testing.T) {
	_, routes := newTestApp()

	tReq := test.CreateRequestTester(
		routes,
		http.MethodPost,
		fmt.Sprintf("/%s/api/rooms/join", "watchparty"),
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(joinRoomForm{RoomCode: ""})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestJoinRoomWithNonExistentCode(t *testing.T) {
	_, routes := newTestApp()

	tReq := test.CreateRequestTester(
		routes,
		http.MethodPost,
		fmt.Sprintf("/%s/api/rooms/join", "watchparty"),
	)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(joinRoomForm{RoomCode: "NONEXISTENT"})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestJoinRoomSuccess(t *testing.T) {
	app, routes := newTestApp()

	roomCode := app.Services.Room.CreateRoom(context.Background(), presenterID)

	tReq := test.CreateRequestTester(
		routes,
		http.MethodPost,
		fmt.Sprintf("/%s/api/rooms/join", "watchparty"),
	)
	tReq.SetFollowRedirect(false)
	tReq.AddCookie(&accessToken)
	tReq.SetContentType(test.FormContentType)
	tReq.SetData(joinRoomForm{RoomCode: roomCode})

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestLeaveRoomWhenNotInRoom(t *testing.T) {
	_, routes := newTestApp()

	tReq := test.CreateRequestTester(
		routes,
		http.MethodGet,
		fmt.Sprintf("/%s/api/rooms/leave", "watchparty"),
	)
	tReq.SetFollowRedirect(false)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestLeaveRoomAsPresenter(t *testing.T) {
	app, routes := newTestApp()

	app.Services.Room.CreateRoom(context.Background(), userID)

	tReq := test.CreateRequestTester(
		routes,
		http.MethodGet,
		fmt.Sprintf("/%s/api/rooms/leave", "watchparty"),
	)
	tReq.SetFollowRedirect(false)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}

func TestLeaveRoomAsViewer(t *testing.T) {
	app, routes := newTestApp()

	roomCode := app.Services.Room.CreateRoom(context.Background(), presenterID)
	app.Services.Room.JoinViewer(context.Background(), roomCode, userID)

	tReq := test.CreateRequestTester(
		routes,
		http.MethodGet,
		fmt.Sprintf("/%s/api/rooms/leave", "watchparty"),
	)
	tReq.SetFollowRedirect(false)
	tReq.AddCookie(&accessToken)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusSeeOther, rs.StatusCode)
}
