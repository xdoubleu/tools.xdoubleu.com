package watchparty

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	config "github.com/xdoubleu/essentia/v3/pkg/config"
	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func (app *WatchParty) templateRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/{$}", prefix),
		app.services.Auth.TemplateAccess(app.rootHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /%s/api/rooms/create", prefix),
		app.services.Auth.Access(app.createRoomHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST /%s/api/rooms/join", prefix),
		app.services.Auth.Access(app.joinRoomHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/api/rooms/leave", prefix),
		app.services.Auth.Access(app.leaveRoomHandler),
	)
}

type rootData struct {
	RoomCode string
}

type lobbyData struct {
	Error string
}

func (app *WatchParty) rootHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	secure := app.config.Env == config.ProdEnv

	if user == nil {
		accessToken, _ := r.Cookie("accessToken")
		aTokenRemoval, rTokenRemoval, _ := app.services.Auth.SignOut(
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

	exists, roomCode, role := app.services.Room.GetRoomForUser(user.ID)
	if !exists {
		// Show lobby where user can create or join a room
		//nolint:exhaustruct // No need to initialize lobbyData with zero values
		tpltools.RenderWithPanic(app.tpl, w, "lobby.html", lobbyData{})
		return
	}

	switch role {
	case dtos.Presenter:
		tpltools.RenderWithPanic(app.tpl, w, "presenter.html", rootData{
			RoomCode: roomCode,
		})
	case dtos.Viewer:
		tpltools.RenderWithPanic(app.tpl, w, "viewer.html", rootData{
			RoomCode: roomCode,
		})
	}
}

func (app *WatchParty) createRoomHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	// If already in a room, redirect to it
	if exists, _, _ := app.services.Room.GetRoomForUser(user.ID); exists {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	app.services.Room.CreateRoom(r.Context(), user.ID)
	http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
}

func (app *WatchParty) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	//nolint:mnd //no magic number
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB

	if err := r.ParseForm(); err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	roomCode := r.FormValue("roomCode")
	if roomCode == "" {
		tpltools.RenderWithPanic(app.tpl, w, "lobby.html", lobbyData{
			Error: "Room code is required.",
		})
		return
	}

	if !app.services.Room.RoomExists(roomCode) {
		tpltools.RenderWithPanic(app.tpl, w, "lobby.html", lobbyData{
			Error: fmt.Sprintf("Room %q does not exist.", roomCode),
		})
		return
	}

	ok := app.services.Room.JoinViewer(r.Context(), roomCode, user.ID)
	if !ok {
		tpltools.RenderWithPanic(app.tpl, w, "lobby.html", lobbyData{
			Error: "Could not join room.",
		})
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

	exists, roomCode, role := app.services.Room.GetRoomForUser(user.ID)
	if !exists {
		http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
		return
	}

	switch role {
	case dtos.Viewer:
		app.services.Room.LeaveViewer(r.Context(), roomCode)
	case dtos.Presenter:
		app.services.Room.RemoveRoom(r.Context(), roomCode)
	}

	http.Redirect(w, r, "/watchparty/", http.StatusSeeOther)
}
