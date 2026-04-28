package backlog

import (
	"net/http"
	"strconv"

	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog/internal/models"
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

type distributionPageData struct {
	Label string
	Games []models.Game
}

func distributionLabels() []string {
	return []string{
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
}

func (app *Backlog) steamPageHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	notStarted, err := app.Services.Steam.GetBacklog(r.Context(), user.ID)
	if err != nil {
		return err
	}

	inProgress, err := app.Services.Steam.GetInProgress(r.Context(), user.ID)
	if err != nil {
		return err
	}

	distribution, _, err := app.Services.Progress.
		GetCompletionRateDistribution(r.Context(), user.ID)
	if err != nil {
		return err
	}

	completed, err := app.Services.Steam.GetCompleted(r.Context(), user.ID)
	if err != nil {
		return err
	}

	currentRate, err := app.Services.Progress.GetCurrentSteamCompletionRate(
		r.Context(),
		user.ID,
	)
	if err != nil {
		return err
	}

	dateStart, dateEnd := parseDateRange(r)
	labels, values, err := app.Services.Progress.GetByTypeIDAndDates(
		r.Context(), models.SteamTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.Tpl, w, "steam.html", steamPageData{
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
	return nil
}

func (app *Backlog) steamDistributionHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	labels := distributionLabels()
	bucket, err := strconv.Atoi(r.PathValue("bucket"))
	if err != nil || bucket < 0 || bucket >= len(labels) {
		http.NotFound(w, r)
		return nil //nolint:nilerr // parse error is handled as 404; don't double-render
	}

	_, bucketGames, err := app.Services.Progress.
		GetCompletionRateDistribution(r.Context(), user.ID)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.Tpl, w, "distribution.html", distributionPageData{
		Label: labels[bucket],
		Games: bucketGames[bucket],
	})
	return nil
}
