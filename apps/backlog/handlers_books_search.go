package backlog

import (
	"net/http"

	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

type searchResultsData struct {
	LibraryResults   []models.UserBook
	HardcoverResults []hardcover.ExternalBook
	FromHardcover    bool
}

type searchLoadingData struct {
	Query string
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
			app.Tpl,
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
			app.Tpl,
			w,
			"books_search_results.html",
			searchResultsData{ //nolint:exhaustruct //other fields are zero value
				LibraryResults: libraryResults,
			},
		)
		return nil
	}

	// No library results — render a loading spinner that triggers the external search.
	tpltools.RenderWithPanic(app.Tpl, w, "books_search_loading.html", searchLoadingData{
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
			app.Tpl,
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
		app.Logger.WarnContext(r.Context(), "hardcover search failed", "error", err)
	}

	tpltools.RenderWithPanic(
		app.Tpl,
		w,
		"books_search_results.html",
		searchResultsData{ //nolint:exhaustruct //LibraryResults is zero value
			HardcoverResults: hardcoverResults,
			FromHardcover:    len(hardcoverResults) > 0,
		},
	)
	return nil
}
