package watchparty

import (
	"errors"
	"fmt"
	"net/http"

	httptools "github.com/XDoubleU/essentia/pkg/communication/http"
	"github.com/XDoubleU/essentia/pkg/context"
	tpltools "github.com/XDoubleU/essentia/pkg/tpl"
	"tools.xdoubleu.com/apps/watchparty/internal/dtos"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func (app *WatchParty) templateRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/{$}", prefix),
		app.services.Auth.TemplateAccess(app.rootHandler),
	)
}

type rootData struct {
	RoomCode string
}

func (app *WatchParty) rootHandler(w http.ResponseWriter, r *http.Request) {
	user := context.GetValue[models.User](r.Context(), constants.UserContextKey)

	if user == nil {
		accessToken, _ := r.Cookie("accessToken")
		aTokenRemoval, rTokenRemoval, _ := app.services.Auth.SignOut(accessToken.Value)
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
		accessToken, _ := r.Cookie("accessToken")
		if accessToken != nil {
			aTokenRemoval, rTokenRemoval, _ := app.services.Auth.SignOut(
				accessToken.Value,
			)
			http.SetCookie(w, aTokenRemoval)
			http.SetCookie(w, rTokenRemoval)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
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
