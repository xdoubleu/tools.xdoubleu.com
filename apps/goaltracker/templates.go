package goaltracker

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	"github.com/xdoubleu/essentia/v3/pkg/parse"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func (app *GoalTracker) templateRoutes(prefix string, mux *http.ServeMux) {
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

	onboarded, err := app.HasCompletedOnboarding(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}
	if !onboarded {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return
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
	StartDate            string
	EndDate              string
	DistributionValues   []int
}

func (app *GoalTracker) graphViewProgress(
	w http.ResponseWriter,
	r *http.Request,
	goal *models.Goal,
	userID string,
) {
	dateStart := goal.PeriodStart()
	dateEnd := goal.PeriodEnd()

	if start := r.URL.Query().Get("start"); start != "" {
		parsedStart, parseErr := time.Parse(models.ProgressDateFormat, start)
		if parseErr == nil {
			dateStart = parsedStart
		}
	}

	if end := r.URL.Query().Get("end"); end != "" {
		parsedEnd, parseErr := time.Parse(models.ProgressDateFormat, end)
		if parseErr == nil {
			dateEnd = parsedEnd
		}
	}

	if dateStart.After(dateEnd) {
		dateStart, dateEnd = dateEnd, dateStart
	}

	progressLabels, progressValues, err := app.Services.Goals.GetProgressByTypeIDAndDates(
		r.Context(),
		*goal.TypeID,
		userID,
		dateStart,
		dateEnd,
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
		Goal:               *goal,
		DateLabels:         progressLabels,
		ProgressValues:     progressValues,
		TargetValues:       []string{},
		CurrentTargetValue: "Unknown",
		Details:            details,
		StartDate:          dateStart.Format(models.ProgressDateFormat),
		EndDate:            dateEnd.Format(models.ProgressDateFormat),
	}

	if len(progressValues) > 0 {
		startProgress, _ := strconv.ParseFloat(progressValues[0], 64)
		targetValues := goal.AdaptiveTargetValuesBetween(
			progressLabels,
			int(startProgress),
		)

		graphData.CurrentProgressValue = progressValues[len(progressValues)-1]

		if dateStart.Equal(goal.PeriodStart()) && dateEnd.Equal(goal.PeriodEnd()) {
			graphData.TargetValues = targetValues
			graphData.CurrentTargetValue = targetValues[len(targetValues)-1]
		}
	}

	if *goal.TypeID == models.SteamCompletionRate.ID {
		distribution, distErr := app.Services.Goals.GetCompletionRateDistribution(
			r.Context(),
			userID,
		)
		if distErr != nil {
			panic(distErr)
		}
		graphData.DistributionValues = distribution
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
