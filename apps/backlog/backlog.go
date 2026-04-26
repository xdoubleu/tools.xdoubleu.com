package backlog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	"github.com/xdoubleu/essentia/v3/pkg/database"
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

type searchResultsData struct {
	LibraryResults   []models.UserBook
	HardcoverResults []hardcover.ExternalBook
	FromHardcover    bool
}

type searchLoadingData struct {
	Query string
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

func groupByStatus(userBooks []models.UserBook) []bookShelf {
	standard := map[string]bool{
		models.StatusToRead:  true,
		models.StatusReading: true,
		models.StatusRead:    true,
		models.StatusDropped: true,
	}
	seen := map[string][]models.UserBook{}
	var order []string
	for _, ub := range userBooks {
		if standard[ub.Status] {
			continue
		}
		if _, ok := seen[ub.Status]; !ok {
			order = append(order, ub.Status)
		}
		seen[ub.Status] = append(seen[ub.Status], ub)
	}
	slices.Sort(order)
	shelves := make([]bookShelf, 0, len(order))
	for _, name := range order {
		shelves = append(shelves, bookShelf{Name: name, Books: seen[name]})
	}
	return shelves
}

func parseRating(raw string) *int16 {
	if raw == "" || raw == "0" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 16)
	if err != nil || n <= 0 {
		return nil
	}
	r16 := int16(n)
	return &r16
}

func currentBacklogUser(r *http.Request) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
}

func (app *Backlog) backlogRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc(
		"GET /"+prefix+"/{$}",
		app.Services.Auth.AppAccess(prefix, app.handle(app.rootHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/users/{userID}",
		app.Services.Auth.AppAccess(prefix, app.handle(app.userBacklogHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/steam",
		app.Services.Auth.AppAccess(prefix, app.handle(app.steamPageHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/steam/distribution/{bucket}",
		app.Services.Auth.AppAccess(prefix, app.handle(app.steamDistributionHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksPageHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/search",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksSearchLibraryHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/search/external",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksSearchExternalHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books",
		app.Services.Auth.AppAccess(prefix, app.handle(app.addBookHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/{id}/status",
		app.Services.Auth.AppAccess(prefix, app.handle(app.updateBookStatusHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/import",
		app.Services.Auth.AppAccess(prefix, app.handle(app.importBooksHandler)),
	)
	mux.HandleFunc(
		"POST /"+prefix+"/books/{id}/tags",
		app.Services.Auth.AppAccess(prefix, app.handle(app.toggleTagHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/library",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksLibraryHandler)),
	)
	mux.HandleFunc(
		"GET /"+prefix+"/books/progress",
		app.Services.Auth.AppAccess(prefix, app.handle(app.booksProgressHandler)),
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

func (app *Backlog) rootHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	onboarded, err := app.HasCompletedOnboarding(r.Context(), user.ID)
	if err != nil {
		return err
	}
	if !onboarded {
		http.Redirect(w, r, "/onboarding", http.StatusSeeOther)
		return nil
	}

	data, err := app.Services.Backlog.GetSummary(r.Context(), user.ID)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.tpl, w, "root.html", data)
	return nil
}

func (app *Backlog) userBacklogHandler(w http.ResponseWriter, r *http.Request) error {
	targetUserID := r.PathValue("userID")

	data, err := app.Services.Backlog.GetSummary(r.Context(), targetUserID)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.tpl, w, "root.html", data)
	return nil
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

	tpltools.RenderWithPanic(app.tpl, w, "distribution.html", distributionPageData{
		Label: labels[bucket],
		Games: bucketGames[bucket],
	})
	return nil
}

func (app *Backlog) booksPageHandler(w http.ResponseWriter, _ *http.Request) error {
	tpltools.RenderWithPanic(app.tpl, w, "books.html", nil)
	return nil
}

func (app *Backlog) booksLibraryHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	library, err := app.Services.Books.GetLibrary(r.Context(), user.ID)
	if err != nil {
		return err
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

	shelves := append(groupByStatus(library), groupByTags(library)...)
	slices.SortFunc(shelves, func(a, b bookShelf) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	tpltools.RenderWithPanic(app.tpl, w, "books_library.html", booksPageData{
		Reading:  reading,
		Wishlist: wishlist,
		Finished: finished,
		Shelves:  shelves,
	})
	return nil
}

func (app *Backlog) booksProgressHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	dateStart, dateEnd := parseDateRange(r)
	labels, values, err := app.Services.Progress.GetByTypeIDAndDates(
		r.Context(), models.BooksTypeID, user.ID, dateStart, dateEnd,
	)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(app.tpl, w, "books_progress.html", booksProgressData{
		Labels:    labels,
		Values:    values,
		DateStart: dateStart.Format(models.ProgressDateFormat),
		DateEnd:   dateEnd.Format(models.ProgressDateFormat),
	})
	return nil
}

// booksSearchLibraryHandler handles the primary HTMX search — library only (fast).
// When no library results are found it renders a loading element that triggers
// booksSearchExternalHandler asynchronously.
func (app *Backlog) booksSearchLibraryHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		tpltools.RenderWithPanic(
			app.tpl,
			w,
			"books_search_results.html",
			searchResultsData{}, //nolint:exhaustruct //empty results
		)
		return nil
	}

	libraryResults, err := app.Services.Books.SearchLibrary(r.Context(), user.ID, query)
	if err != nil {
		return err
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
		return nil
	}

	// No library results — render a loading spinner that triggers the external search.
	tpltools.RenderWithPanic(app.tpl, w, "books_search_loading.html", searchLoadingData{
		Query: query,
	})
	return nil
}

// booksSearchExternalHandler is called asynchronously by the loading spinner when
// the library search returned no results. It hits the Hardcover API.
func (app *Backlog) booksSearchExternalHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		tpltools.RenderWithPanic(
			app.tpl,
			w,
			"books_search_results.html",
			searchResultsData{}, //nolint:exhaustruct //empty results
		)
		return nil
	}

	hardcoverResults, err := app.Services.Books.SearchHardcover(
		r.Context(),
		user.ID,
		query,
	)
	if err != nil {
		// Log but show empty results — external API may be unavailable.
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
	return nil
}

func (app *Backlog) addBookHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	var dto dtos.AddBookDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return httpError(http.StatusBadRequest, "Invalid form data")
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

	initialTags := []string{}
	if dto.OwnPhysical {
		initialTags = append(initialTags, models.TagOwnPhysical)
	}
	if dto.OwnDigital {
		initialTags = append(initialTags, models.TagOwnDigital)
	}

	if _, err := app.Services.Books.AddToLibrary(
		r.Context(), user.ID, ext, dto.Status, initialTags,
	); err != nil {
		return err
	}

	http.Redirect(w, r, "/backlog/books", http.StatusSeeOther)
	return nil
}

func (app *Backlog) updateBookStatusHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	bookID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return nil //nolint:nilerr // parse error is handled as 404; don't double-render
	}

	var dto dtos.UpdateBookStatusDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return httpError(http.StatusBadRequest, "Invalid form data")
	}

	// Fetch existing entry to preserve tags and other fields.
	existing, err := app.Services.Books.GetUserBook(r.Context(), user.ID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return err
	}

	var existingTags []string
	if existing != nil {
		existingTags = existing.Tags
	}

	existingTags = toggleTag(existingTags, models.TagFavourite, dto.Favourite)

	rating := parseRating(dto.Rating)

	var notes *string
	if dto.Notes != "" {
		notes = &dto.Notes
	}

	var finishedAt []time.Time
	if dto.Status == models.StatusRead {
		if existing != nil {
			finishedAt = append(finishedAt, existing.FinishedAt...)
			if existing.Status != models.StatusRead {
				finishedAt = append(finishedAt, time.Now())
			}
		} else {
			finishedAt = append(finishedAt, time.Now())
		}
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
		return err
	}

	if dto.Status == models.StatusRead {
		labels, values, buildErr := app.Services.Books.BuildReadProgress(
			r.Context(),
			user.ID,
		)
		if buildErr != nil {
			return buildErr
		}
		if saveErr := app.Services.Progress.Save(
			r.Context(), models.BooksTypeID, user.ID, labels, values,
		); saveErr != nil {
			return saveErr
		}
	}

	http.Redirect(w, r, "/backlog/books", http.StatusSeeOther)
	return nil
}

func (app *Backlog) toggleTagHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	bookID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return nil //nolint:nilerr // parse error is handled as 404; don't double-render
	}

	var dto dtos.ToggleTagDto
	if err = httptools.ReadForm(r, &dto); err != nil || dto.Tag == "" {
		return httpError(http.StatusBadRequest, "Invalid form data")
	}

	if err = app.Services.Books.ToggleTag(
		r.Context(), user.ID, bookID, dto.Tag,
	); err != nil {
		return err
	}

	http.Redirect(w, r, "/backlog/books", http.StatusSeeOther)
	return nil
}

func (app *Backlog) importBooksHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentBacklogUser(r)
	if user == nil {
		return httpError(http.StatusUnauthorized, "Sign in to access this page")
	}

	const maxUploadBytes = 10 << 20 // 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return httpError(http.StatusBadRequest, "File is too large (max 10 MB)")
	}

	file, _, err := r.FormFile("goodreads_csv")
	if err != nil {
		return httpError(http.StatusBadRequest, "Missing CSV file")
	}
	defer file.Close()

	// Detach from the HTTP request deadline: importing a large CSV can take
	// longer than the server's read/write timeout, and the DB batch work must
	// complete even if the connection deadline fires.
	importCtx := context.WithoutCancel(r.Context())

	count, err := app.Services.Books.ImportFromCSV(importCtx, user.ID, file)
	if err != nil {
		return err
	}

	labels, values, err := app.Services.Books.BuildReadProgress(importCtx, user.ID)
	if err != nil {
		return err
	}
	if err = app.Services.Progress.Save(
		importCtx, models.BooksTypeID, user.ID, labels, values,
	); err != nil {
		return err
	}

	http.Redirect(
		w,
		r,
		fmt.Sprintf("/backlog/books?imported=%d", count),
		http.StatusSeeOther,
	)
	return nil
}

func (app *Backlog) refreshHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	app.Services.WebSocket.ForceRun(id)
	w.WriteHeader(http.StatusNoContent)
}
