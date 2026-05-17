package watchparty

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	config "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func (app *WatchParty) templateRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/{$}", prefix),
		app.Services.Auth.AppAccess(prefix, app.rootHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /%s/api/rooms/create", prefix),
		app.Services.Auth.Access(app.createRoomHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /%s/api/rooms/join", prefix),
		app.Services.Auth.Access(app.joinRoomHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/api/rooms/leave", prefix),
		app.Services.Auth.Access(app.leaveRoomHandler),
	)
}

func (app *WatchParty) rootHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	secure := app.Config.Env == config.ProdEnv

	if user == nil {
		accessToken, _ := r.Cookie("accessToken")
		aTokenRemoval, rTokenRemoval, _ := app.Services.Auth.SignOut(
			accessToken.Value,
			secure,
		)
		http.SetCookie(w, aTokenRemoval)
		http.SetCookie(w, rTokenRemoval)
		httptools.RedirectWithError(
			w,
			r,
			"/",
			errors.New("unable to get user from context"),
		)
		return
	}

	exists, roomCode, role := app.Services.Room.GetRoomForUser(user.ID)
	if !exists {
		_ = LobbyPage(LobbyViewData{Error: ""}).Render(r.Context(), w)
		return
	}

	switch role {
	case dtos.Presenter:
		_ = RoomPage(RoomViewData{
			RoomCode:    roomCode,
			IsPresenter: true,
		}).Render(r.Context(), w)
	case dtos.Viewer:
		_ = RoomPage(RoomViewData{
			RoomCode:    roomCode,
			IsPresenter: false,
		}).Render(r.Context(), w)
	}
}

func (app *WatchParty) createRoomHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	// If already in a room, redirect to it
	if exists, _, _ := app.Services.Room.GetRoomForUser(user.ID); exists {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	app.Services.Room.CreateRoom(r.Context(), user.ID)
	http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
}

func (app *WatchParty) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	var dto dtos.JoinRoomDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		httptools.HandleError(w, r, err)
		return
	}

	if ok, errs := dto.Validate(); !ok {
		httptools.FailedValidationResponse(w, r, errs)
		return
	}

	if !app.Services.Room.RoomExists(dto.RoomCode) {
		_ = LobbyPage(LobbyViewData{
			Error: fmt.Sprintf("Room %q does not exist.", dto.RoomCode),
		}).Render(r.Context(), w)
		return
	}

	ok := app.Services.Room.JoinViewer(r.Context(), dto.RoomCode, user.ID)
	if !ok {
		_ = LobbyPage(LobbyViewData{
			Error: "Could not join room.",
		}).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
}

func (app *WatchParty) leaveRoomHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	exists, roomCode, role := app.Services.Room.GetRoomForUser(user.ID)
	if !exists {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	switch role {
	case dtos.Viewer:
		app.Services.Room.LeaveViewer(r.Context(), roomCode)
	case dtos.Presenter:
		app.Services.Room.RemoveRoom(r.Context(), roomCode)
	}

	http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
}
