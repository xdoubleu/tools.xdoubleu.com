package goaltracker

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/xdoubleu/essentia/v2/pkg/contexttools"
	"github.com/xdoubleu/essentia/v2/pkg/parse"
	tpltools "github.com/xdoubleu/essentia/v2/pkg/tpl"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (app *GoalTracker) templateRoutes(prefix string, mux *http.ServeMux) {
	mux.Handle(
		fmt.Sprintf("GET /%s/images/", prefix),
		http.FileServerFS(app.images),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/{$}", prefix),
		app.Services.Auth.TemplateAccess(app.rootHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/edit/{id}", prefix),
		app.Services.Auth.TemplateAccess(app.editHandler),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET /%s/goals/{id}", prefix),
		app.Services.Auth.TemplateAccess(app.goalProgressHandler),
	)
}

func (app *GoalTracker) rootHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	goals, err := app.Services.Goals.GetAllGoalsGroupedByStateAndParentGoal(
		r.Context(),
		user.ID,
	)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "root.html", goals)
}

type LinkTemplateData struct {
	Goal    models.Goal
	Sources []models.Source
	Tags    []string
}

func (app *GoalTracker) editHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, err := parse.URLParam[string](r, "id", nil)
	if err != nil {
		panic(err)
	}

	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	goal, err := app.Services.Goals.GetGoalByID(r.Context(), id, user.ID)
	if err != nil {
		panic(err)
	}

	tags, err := app.Services.Goodreads.GetAllTags(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	goalAndSources := LinkTemplateData{
		Goal:    *goal,
		Sources: models.Sources,
		Tags:    tags,
	}

	tpltools.RenderWithPanic(app.tpl, w, "edit.html", goalAndSources)
}

func (app *GoalTracker) goalProgressHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parse.URLParam[string](r, "id", nil)
	if err != nil {
		panic(err)
	}

	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	goal, err := app.Services.Goals.GetGoalByID(r.Context(), id, user.ID)
	if err != nil {
		panic(err)
	}

	viewType := models.Types[*goal.TypeID].ViewType
	switch viewType {
	case models.Graph:
		app.graphViewProgress(w, r, goal, user.ID)
	case models.List:
		app.listViewProgress(w, r, goal, user.ID)
	}
}

type GraphData struct {
	Goal                 models.Goal
	DateLabels           []string
	ProgressValues       []string
	TargetValues         []string
	CurrentProgressValue string
	CurrentTargetValue   string
	Details              []models.ListItem
}

func (app *GoalTracker) graphViewProgress(
	w http.ResponseWriter,
	r *http.Request,
	goal *models.Goal,
	userID string,
) {
	progressLabels, progressValues, err := app.Services.Goals.GetProgressByTypeIDAndDates(
		r.Context(),
		*goal.TypeID,
		userID,
		goal.PeriodStart(),
		goal.PeriodEnd(),
	)
	if err != nil {
		panic(err)
	}

	details, err := app.Services.Goals.GetListItemsByGoal(r.Context(), goal, userID)
	if err != nil {
		panic(err)
	}

	//nolint:exhaustruct //others are defined later
	graphData := GraphData{
		Goal:           *goal,
		DateLabels:     progressLabels,
		ProgressValues: progressValues,
		Details:        details,
	}

	if len(progressValues) > 0 {
		startProgress, _ := strconv.ParseFloat(progressValues[0], 64)
		graphData.TargetValues = goal.AdaptiveTargetValues(int(startProgress))
		graphData.CurrentProgressValue = progressValues[len(progressValues)-1]
		graphData.CurrentTargetValue = graphData.TargetValues[len(progressValues)-1]
	}

	tpltools.RenderWithPanic(app.tpl, w, "graph.html", graphData)
}

type ListData struct {
	Goal      models.Goal
	ListItems []models.ListItem
}

func (app *GoalTracker) listViewProgress(
	w http.ResponseWriter,
	r *http.Request,
	goal *models.Goal,
	userID string,
) {
	listItems, err := app.Services.Goals.GetListItemsByGoal(
		r.Context(),
		goal,
		userID,
	)
	if err != nil {
		panic(err)
	}

	listData := ListData{
		Goal:      *goal,
		ListItems: listItems,
	}

	tpltools.RenderWithPanic(app.tpl, w, "list.html", listData)
}
