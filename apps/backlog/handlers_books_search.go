package backlog

import (
	"net/http"

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
		_ = BooksSearchResultsPage(
			searchResultsData{}, //nolint:exhaustruct //empty results
		).Render(r.Context(), w)
		return nil
	}

	libraryResults, err := app.Services.Books.SearchLibrary(r.Context(), user.ID, query)
	if err != nil {
		return err
	}
	if len(libraryResults) > 0 {
		_ = BooksSearchResultsPage(
			searchResultsData{ //nolint:exhaustruct //other fields are zero value
				LibraryResults: libraryResults,
			},
		).Render(r.Context(), w)
		return nil
	}

	// No library results — render a loading spinner that triggers the external search.
	_ = BooksSearchLoadingPage(searchLoadingData{
		Query: query,
	}).Render(r.Context(), w)
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
		_ = BooksSearchResultsPage(
			searchResultsData{}, //nolint:exhaustruct //empty results
		).Render(r.Context(), w)
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

	_ = BooksSearchResultsPage(
		searchResultsData{ //nolint:exhaustruct //LibraryResults is zero value
			HardcoverResults: hardcoverResults,
			FromHardcover:    len(hardcoverResults) > 0,
		},
	).Render(r.Context(), w)
	return nil
}
