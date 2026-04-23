package backlog

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/goodreads"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type steamPageData struct {
	NotStarted   []models.Game
	InProgress   []models.Game
	Completed    []models.Game
	TotalBacklog int
	Distribution []int
	Labels       []string
	Values       []string
	CurrentRate  string
	DateStart    string
	DateEnd      string
}

var distLabels = []string{ //nolint:gochecknoglobals //shared by handlers
	"0–9%",
	"10–19%",
	"20–29%",
	"30–39%",
	"40–49%",
	"50–59%",
	"60–69%",
	"70–79%",
	"80–89%",
	"90–99%",
	"100%",
}

type distributionPageData struct {
	Label string
	Games []models.Game
}

type goodreadsPageData struct {
	Books     []goodreads.Book
	Labels    []string
	Values    []string
	DateStart string
	DateEnd   string
}

func (app *Backlog) backlogRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/{$}",
		app.Services.Auth.AppAccess(prefix, app.rootHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/users/{userID}",
		app.Services.Auth.AppAccess(prefix, app.userBacklogHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/steam",
		app.Services.Auth.AppAccess(prefix, app.steamPageHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/steam/distribution/{bucket}",
		app.Services.Auth.AppAccess(prefix, app.steamDistributionHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/goodreads",
		app.Services.Auth.AppAccess(prefix, app.goodreadsPageHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress",
		app.Services.WebSocket.Handler(),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/api/progress/{id}/refresh",
		app.refreshHandler,
	)
}

func (app *Backlog) Routes(prefix string, mux *http.ServeMux) {
	app.backlogRoutes(prefix, mux)
}

func (app *Backlog) rootHandler(w http.ResponseWriter, r *http.Request) {
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

	data, err := app.Services.Backlog.GetSummary(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "root.html", data)
}

func (app *Backlog) userBacklogHandler(w http.ResponseWriter, r *http.Request) {
	targetUserID := r.PathValue("userID")

	data, err := app.Services.Backlog.GetSummary(r.Context(), targetUserID)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "root.html", data)
}

func parseDateRange(r *http.Request) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(models.ProgressDateFormat, v); err == nil {
			start = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(models.ProgressDateFormat, v); err == nil {
			end = t
		}
	}
	return start, end
}

func (app *Backlog) steamPageHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	notStarted, err := app.Services.Steam.GetBacklog(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	inProgress, err := app.Services.Steam.GetInProgress(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	distribution, _, err := app.Services.Progress.
		GetCompletionRateDistribution(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	completed, err := app.Services.Steam.GetCompleted(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	currentRate, err := app.Services.Progress.GetCurrentSteamCompletionRate(
		r.Context(),
		user.ID,
	)
	if err != nil {
		panic(err)
	}

	dateStart, dateEnd := parseDateRange(r)
	labels, values, err := app.Services.Progress.GetByTypeIDAndDates(
		r.Context(), models.SteamTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "steam.html", steamPageData{
		NotStarted:   notStarted,
		InProgress:   inProgress,
		Completed:    completed,
		TotalBacklog: len(notStarted) + len(inProgress),
		Distribution: distribution,
		CurrentRate:  currentRate,
		Labels:       labels,
		Values:       values,
		DateStart:    dateStart.Format(models.ProgressDateFormat),
		DateEnd:      dateEnd.Format(models.ProgressDateFormat),
	})
}

func (app *Backlog) steamDistributionHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	bucket, err := strconv.Atoi(r.PathValue("bucket"))
	if err != nil || bucket < 0 || bucket >= len(distLabels) {
		http.NotFound(w, r)
		return
	}

	_, bucketGames, err := app.Services.Progress.
		GetCompletionRateDistribution(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "distribution.html", distributionPageData{
		Label: distLabels[bucket],
		Games: bucketGames[bucket],
	})
}

func (app *Backlog) goodreadsPageHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	books, err := app.Services.Goodreads.GetWantToRead(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	dateStart, dateEnd := parseDateRange(r)
	labels, values, err := app.Services.Progress.GetByTypeIDAndDates(
		r.Context(), models.GoodreadsTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "goodreads.html", goodreadsPageData{
		Books:     books,
		Labels:    labels,
		Values:    values,
		DateStart: dateStart.Format(models.ProgressDateFormat),
		DateEnd:   dateEnd.Format(models.ProgressDateFormat),
	})
}

func (app *Backlog) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app.Services.WebSocket.ForceRun(id)
	w.WriteHeader(http.StatusNoContent)
}
