package backlog

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
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

type booksPageData struct {
	Wishlist      []models.UserBook
	Reading       []models.UserBook
	Finished      []models.UserBook
	Labels        []string
	Values        []string
	DateStart     string
	DateEnd       string
	ImportedCount *int
}

type searchResultsData struct {
	Results []hardcover.ExternalBook
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
		"GET /"+prefix+"/books",
		app.Services.Auth.AppAccess(prefix, app.booksPageHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/search",
		app.Services.Auth.AppAccess(prefix, app.booksSearchHandler),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books",
		app.Services.Auth.AppAccess(prefix, app.addBookHandler),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/{id}/status",
		app.Services.Auth.AppAccess(prefix, app.updateBookStatusHandler),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/import",
		app.Services.Auth.AppAccess(prefix, app.importBooksHandler),
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

func (app *Backlog) booksPageHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	wishlist, err := app.Services.Books.GetByStatus(r.Context(), user.ID, models.StatusWishlist)
	if err != nil {
		panic(err)
	}

	reading, err := app.Services.Books.GetByStatus(r.Context(), user.ID, models.StatusReading)
	if err != nil {
		panic(err)
	}

	finished, err := app.Services.Books.GetByStatus(r.Context(), user.ID, models.StatusFinished)
	if err != nil {
		panic(err)
	}

	dateStart, dateEnd := parseDateRange(r)
	labels, values, err := app.Services.Progress.GetByTypeIDAndDates(
		r.Context(), models.BooksTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "books.html", booksPageData{
		Wishlist:  wishlist,
		Reading:   reading,
		Finished:  finished,
		Labels:    labels,
		Values:    values,
		DateStart: dateStart.Format(models.ProgressDateFormat),
		DateEnd:   dateEnd.Format(models.ProgressDateFormat),
	})
}

func (app *Backlog) booksSearchHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		tpltools.RenderWithPanic(app.tpl, w, "books_search_results.html", searchResultsData{})
		return
	}

	results, err := app.Services.Books.Search(r.Context(), user.ID, query)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "books_search_results.html", searchResultsData{
		Results: results,
	})
}

func (app *Backlog) addBookHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	var dto struct {
		ProviderID  string `schema:"provider_id"`
		Provider    string `schema:"provider"`
		Title       string `schema:"title"`
		Author      string `schema:"author"`
		ISBN13      string `schema:"isbn13"`
		CoverURL    string `schema:"cover_url"`
		Description string `schema:"description"`
		Status      string `schema:"status"`
	}
	if err := httptools.ReadForm(r, &dto); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	if dto.Status == "" {
		dto.Status = models.StatusWishlist
	}

	var isbn13 *string
	if dto.ISBN13 != "" {
		isbn13 = &dto.ISBN13
	}
	var coverURL *string
	if dto.CoverURL != "" {
		coverURL = &dto.CoverURL
	}
	var desc *string
	if dto.Description != "" {
		desc = &dto.Description
	}

	ext := hardcover.ExternalBook{
		Provider:    dto.Provider,
		ProviderID:  dto.ProviderID,
		Title:       dto.Title,
		Authors:     []string{dto.Author},
		ISBN13:      isbn13,
		CoverURL:    coverURL,
		Description: desc,
	}

	if _, err := app.Services.Books.AddToLibrary(r.Context(), user.ID, ext, dto.Status); err != nil {
		panic(err)
	}

	http.Redirect(w, r, "/backlog/books", http.StatusSeeOther)
}

func (app *Backlog) updateBookStatusHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	bookID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var dto struct {
		Status string `schema:"status"`
	}
	if err = httptools.ReadForm(r, &dto); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:     user.ID,
		BookID:     bookID,
		Status:     dto.Status,
		FinishedAt: []time.Time{time.Now()},
	}
	if err = app.Services.Books.UpdateStatus(r.Context(), user.ID, ub); err != nil {
		panic(err)
	}

	// After marking finished, rebuild and save progress.
	if dto.Status == models.StatusFinished {
		labels, values, buildErr := app.Services.Books.BuildReadProgress(r.Context(), user.ID)
		if buildErr != nil {
			panic(buildErr)
		}
		if saveErr := app.Services.Progress.Save(
			r.Context(), models.BooksTypeID, user.ID, labels, values,
		); saveErr != nil {
			panic(saveErr)
		}
	}

	http.Redirect(w, r, "/backlog/books", http.StatusSeeOther)
}

func (app *Backlog) importBooksHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	const maxUploadBytes = 10 << 20 // 10 MB
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "file too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("goodreads_csv")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	count, err := app.Services.Books.ImportFromCSV(r.Context(), user.ID, file)
	if err != nil {
		panic(err)
	}

	// Rebuild progress after bulk import.
	labels, values, err := app.Services.Books.BuildReadProgress(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}
	if err = app.Services.Progress.Save(
		r.Context(), models.BooksTypeID, user.ID, labels, values,
	); err != nil {
		panic(err)
	}

	http.Redirect(w, r, fmt.Sprintf("/backlog/books?imported=%d", count), http.StatusSeeOther)
}

func (app *Backlog) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app.Services.WebSocket.ForceRun(id)
	w.WriteHeader(http.StatusNoContent)
}
