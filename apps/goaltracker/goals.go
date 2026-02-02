package goaltracker

import (
	"errors"
	"fmt"
	"net/http"

	httptools "github.com/xdoubleu/essentia/v2/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v2/pkg/contexttools"
	"github.com/xdoubleu/essentia/v2/pkg/parse"
	"tools.xdoubleu.com/apps/goaltracker/internal/dtos"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func (app *GoalTracker) goalsRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		fmt.Sprintf("POST %s/goals/{id}/edit", prefix),
		app.Services.Auth.Access(app.editGoalHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/goals/{id}/unlink", prefix),
		app.Services.Auth.Access(app.unlinkGoalHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/goals/{id}/complete", prefix),
		app.Services.Auth.Access(app.completeGoalHandler),
	)
}

func (app *GoalTracker) editGoalHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parse.URLParam[string](r, "id", nil)
	if err != nil {
		panic(err)
	}

	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	var linkGoalDto dtos.LinkGoalDto

	err = httptools.ReadForm(r, &linkGoalDto)
	if err != nil {
		httptools.RedirectWithError(w, r, fmt.Sprintf("/edit/%s", id), err)
		return
	}

	if ok, errs := linkGoalDto.Validate(); !ok {
		httptools.FailedValidationResponse(w, r, errs)
		return
	}

	err = app.Services.Goals.LinkGoal(r.Context(), id, user.ID, &linkGoalDto)
	if err != nil {
		httptools.RedirectWithError(w, r, fmt.Sprintf("/edit/%s", id), err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/goals/%s", id), http.StatusSeeOther)
}

func (app *GoalTracker) unlinkGoalHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parse.URLParam[string](r, "id", nil)
	if err != nil {
		panic(err)
	}

	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	err = app.Services.Goals.UnlinkGoal(r.Context(), id, user.ID)
	if err != nil {
		panic(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *GoalTracker) completeGoalHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parse.URLParam[string](r, "id", nil)
	if err != nil {
		panic(err)
	}

	user := contexttools.GetValue[models.User](r.Context(), constants.UserContextKey)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	err = app.Services.Goals.CompleteGoal(r.Context(), id, user.ID)
	if err != nil {
		panic(err)
	}

	err = app.Services.Goals.ImportGoalsFromTodoist(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
