package watchparty_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xdoubleu/essentia/v3/pkg/test"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
)

const presenterID = "presenter-user-id-for-testing"

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
	tReq.SetData(dtos.JoinRoomDto{RoomCode: ""})

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
	tReq.SetData(dtos.JoinRoomDto{RoomCode: "NONEXISTENT"})

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
	tReq.SetData(dtos.JoinRoomDto{RoomCode: roomCode})

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

func roomHTML(t *testing.T, routes http.Handler) string {
	t.Helper()
	tReq := test.CreateRequestTester(
		routes,
		http.MethodGet,
		fmt.Sprintf("/%s/", "watchparty"),
	)
	tReq.AddCookie(&accessToken)
	rs := tReq.Do(t)
	body, err := io.ReadAll(rs.Body)
	assert.NoError(t, err)
	return string(body)
}

func TestRoomPresenterContainsControls(t *testing.T) {
	app, routes := newTestApp()
	app.Services.Room.CreateRoom(context.Background(), userID)

	body := roomHTML(t, routes)

	assert.Contains(t, body, "Mute Mic")
	assert.Contains(t, body, "Disable Cam")
	assert.Contains(t, body, "Hide Self")
	assert.NotContains(t, body, "Stream vol")
}

func TestRoomViewerContainsControls(t *testing.T) {
	app, routes := newTestApp()
	roomCode := app.Services.Room.CreateRoom(context.Background(), presenterID)
	app.Services.Room.JoinViewer(context.Background(), roomCode, userID)

	body := roomHTML(t, routes)

	assert.Contains(t, body, "Mute Mic")
	assert.Contains(t, body, "Disable Cam")
	assert.Contains(t, body, "Hide Self")
	assert.Contains(t, body, "Stream vol")
}

func TestRoomPresenterContainsReconnectLogic(t *testing.T) {
	app, routes := newTestApp()
	app.Services.Room.CreateRoom(context.Background(), userID)

	body := roomHTML(t, routes)

	// Presenter re-initiates screen share when a stale answer arrives
	assert.Contains(t, body, "isSharingScreen && localScreen")
}

func TestRoomContainsStaleOfferGuard(t *testing.T) {
	app, routes := newTestApp()
	app.Services.Room.CreateRoom(context.Background(), userID)

	body := roomHTML(t, routes)

	// Failed/closed screen PC is replaced before accepting a new remote offer
	assert.Contains(t, body, `pc.connectionState === "failed"`)
}

func TestRoomViewerAudioMixingUsesScreenStream(t *testing.T) {
	app, routes := newTestApp()
	roomCode := app.Services.Room.CreateRoom(context.Background(), presenterID)
	app.Services.Room.JoinViewer(context.Background(), roomCode, userID)

	body := roomHTML(t, routes)

	// Viewer applyVol only affects mainVideoEl when screen is being shared
	assert.Contains(t, body, "isSharingScreen")
	// remoteCamEl volume is never touched by the duck/vol controls
	assert.NotContains(t, body, "remoteCamEl.volume")
}
