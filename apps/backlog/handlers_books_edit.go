package backlog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"tools.xdoubleu.com/apps/backlog/internal/dtos"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
)

func isHXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func buildFinishedAt(existing *models.UserBook, newStatus string) []time.Time {
	if newStatus != models.StatusRead {
		return nil
	}
	if existing == nil {
		return []time.Time{time.Now()}
	}
	result := append([]time.Time{}, existing.FinishedAt...)
	if existing.Status != models.StatusRead {
		result = append(result, time.Now())
	}
	return result
}

func (app *Backlog) rebuildReadProgress(ctx context.Context, userID string) error {
	labels, values, err := app.Services.Books.BuildReadProgress(ctx, userID)
	if err != nil {
		return err
	}
	return app.Services.Progress.Save(ctx, models.BooksTypeID, userID, labels, values)
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

	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:     user.ID,
		BookID:     bookID,
		Status:     dto.Status,
		Tags:       existingTags,
		Rating:     rating,
		Notes:      notes,
		FinishedAt: buildFinishedAt(existing, dto.Status),
	}
	if err = app.Services.Books.UpdateStatus(r.Context(), user.ID, ub); err != nil {
		return err
	}

	if dto.Status == models.StatusRead {
		if rebuildErr := app.rebuildReadProgress(r.Context(), user.ID); rebuildErr != nil {
			return rebuildErr
		}
	}

	if isHXRequest(r) {
		data, libErr := app.buildLibraryData(r, user.ID)
		if libErr != nil {
			return libErr
		}
		_ = BooksLibraryPage(data).Render(r.Context(), w)
		return nil
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

	if isHXRequest(r) {
		data, libErr := app.buildLibraryData(r, user.ID)
		if libErr != nil {
			return libErr
		}
		_ = BooksLibraryPage(data).Render(r.Context(), w)
		return nil
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

	if err = app.rebuildReadProgress(importCtx, user.ID); err != nil {
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
