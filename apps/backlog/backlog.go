package backlog

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog/internal/dtos"
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

type bookShelf struct {
	Name  string
	Books []models.UserBook
}

type booksPageData struct {
	Reading  []models.UserBook
	Wishlist []models.UserBook
	Finished []models.UserBook
	Shelves  []bookShelf
}

type booksProgressData struct {
	Labels    []string
	Values    []string
	DateStart string
	DateEnd   string
}

func toggleTag(tags []string, tag string, enable bool) []string {
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != tag {
			result = append(result, t)
		}
	}
	if enable {
		result = append(result, tag)
	}
	return result
}

func groupByTags(userBooks []models.UserBook) []bookShelf {
	seen := map[string][]models.UserBook{}
	var order []string
	for _, ub := range userBooks {
		for _, tag := range ub.Tags {
			if models.IsSpecialTag(tag) {
				continue
			}
			if _, ok := seen[tag]; !ok {
				order = append(order, tag)
			}
			seen[tag] = append(seen[tag], ub)
		}
	}
	slices.Sort(order)
	shelves := make([]bookShelf, 0, len(order))
	for _, name := range order {
		shelves = append(shelves, bookShelf{Name: name, Books: seen[name]})
	}
	return shelves
}

type searchResultsData struct {
	LibraryResults   []models.UserBook
	HardcoverResults []hardcover.ExternalBook
	FromHardcover    bool
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
		"POST /"+prefix+"/books/{id}/tags",
		app.Services.Auth.AppAccess(prefix, app.toggleTagHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/library",
		app.Services.Auth.AppAccess(prefix, app.booksLibraryHandler),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/progress",
		app.Services.Auth.AppAccess(prefix, app.booksProgressHandler),
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

func (app *Backlog) booksPageHandler(w http.ResponseWriter, _ *http.Request) {
	tpltools.RenderWithPanic(app.tpl, w, "books.html", nil)
}

func (app *Backlog) booksLibraryHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	library, err := app.Services.Books.GetLibrary(r.Context(), user.ID)
	if err != nil {
		panic(err)
	}

	var reading, wishlist, finished []models.UserBook
	for _, ub := range library {
		switch ub.Status {
		case models.StatusReading:
			reading = append(reading, ub)
		case models.StatusToRead:
			wishlist = append(wishlist, ub)
		case models.StatusRead:
			finished = append(finished, ub)
		}
	}

	tpltools.RenderWithPanic(app.tpl, w, "books_library.html", booksPageData{
		Reading:  reading,
		Wishlist: wishlist,
		Finished: finished,
		Shelves:  groupByTags(library),
	})
}

func (app *Backlog) booksProgressHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	dateStart, dateEnd := parseDateRange(r)
	labels, values, err := app.Services.Progress.GetByTypeIDAndDates(
		r.Context(), models.BooksTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		panic(err)
	}

	tpltools.RenderWithPanic(app.tpl, w, "books_progress.html", booksProgressData{
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
		tpltools.RenderWithPanic(
			app.tpl,
			w,
			"books_search_results.html",
			searchResultsData{
				LibraryResults:   nil,
				HardcoverResults: nil,
				FromHardcover:    false,
			},
		)
		return
	}

	// Search library first; fall back to Hardcover only when library has no matches.
	libraryResults, err := app.Services.Books.SearchLibrary(r.Context(), user.ID, query)
	if err != nil {
		panic(err)
	}
	if len(libraryResults) > 0 {
		tpltools.RenderWithPanic(
			app.tpl,
			w,
			"books_search_results.html",
			searchResultsData{ //nolint:exhaustruct //other fields are zero value
				LibraryResults: libraryResults,
			},
		)
		return
	}

	hardcoverResults, err := app.Services.Books.SearchHardcover(
		r.Context(),
		user.ID,
		query,
	)
	if err != nil {
		// Log but don't fail the page — external API may be slow or unavailable.
		app.logger.WarnContext(r.Context(), "hardcover search failed", "error", err)
	}
	tpltools.RenderWithPanic(
		app.tpl,
		w,
		"books_search_results.html",
		searchResultsData{ //nolint:exhaustruct //LibraryResults is zero value
			HardcoverResults: hardcoverResults,
			FromHardcover:    len(hardcoverResults) > 0,
		},
	)
}

func (app *Backlog) addBookHandler(w http.ResponseWriter, r *http.Request) {
	user := contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
	if user == nil {
		panic(errors.New("not signed in"))
	}

	var dto dtos.AddBookDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	if dto.Status == "" {
		dto.Status = models.StatusToRead
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
		ISBN10:      nil,
		CoverURL:    coverURL,
		Description: desc,
	}

	var initialTags []string
	if dto.OwnPhysical {
		initialTags = append(initialTags, models.TagOwnPhysical)
	}
	if dto.OwnDigital {
		initialTags = append(initialTags, models.TagOwnDigital)
	}

	if _, err := app.Services.Books.AddToLibrary(
		r.Context(), user.ID, ext, dto.Status, initialTags,
	); err != nil {
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

	var dto dtos.UpdateBookStatusDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	// Fetch existing entry to preserve tags and other fields.
	existing, err := app.Services.Books.GetUserBook(r.Context(), user.ID, bookID)
	if err != nil {
		panic(err)
	}

	var existingTags []string
	if existing != nil {
		existingTags = existing.Tags
	}

	// Toggle favourite tag.
	existingTags = toggleTag(existingTags, models.TagFavourite, dto.Favourite)

	var rating *int16
	if dto.Rating != "" && dto.Rating != "0" {
		if n, parseErr := strconv.ParseInt(dto.Rating, 10, 16); parseErr == nil &&
			n > 0 {
			r16 := int16(n)
			rating = &r16
		}
	}

	var notes *string
	if dto.Notes != "" {
		notes = &dto.Notes
	}

	var finishedAt []time.Time
	if dto.Status == models.StatusRead {
		if existing != nil {
			finishedAt = append(finishedAt, existing.FinishedAt...)
		}
		finishedAt = append(finishedAt, time.Now())
	}

	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:     user.ID,
		BookID:     bookID,
		Status:     dto.Status,
		Tags:       existingTags,
		Rating:     rating,
		Notes:      notes,
		FinishedAt: finishedAt,
	}
	if err = app.Services.Books.UpdateStatus(r.Context(), user.ID, ub); err != nil {
		panic(err)
	}

	// After marking finished, rebuild and save progress.
	if dto.Status == models.StatusRead {
		labels, values, buildErr := app.Services.Books.BuildReadProgress(
			r.Context(),
			user.ID,
		)
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

func (app *Backlog) toggleTagHandler(w http.ResponseWriter, r *http.Request) {
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

	var dto dtos.ToggleTagDto
	if err = httptools.ReadForm(r, &dto); err != nil || dto.Tag == "" {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	if err = app.Services.Books.ToggleTag(
		r.Context(), user.ID, bookID, dto.Tag,
	); err != nil {
		panic(err)
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
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
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

	http.Redirect(
		w,
		r,
		fmt.Sprintf("/backlog/books?imported=%d", count),
		http.StatusSeeOther,
	)
}

func (app *Backlog) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app.Services.WebSocket.ForceRun(id)
	w.WriteHeader(http.StatusNoContent)
}
