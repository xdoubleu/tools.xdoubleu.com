package services

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/internal/services/conversion"
	"tools.xdoubleu.com/apps/books/kobo"
	"tools.xdoubleu.com/apps/books/reader/positions"
	"tools.xdoubleu.com/internal/kit/database"
)

// ErrSeriesNameRequired is returned for a blank series name.
var ErrSeriesNameRequired = errors.New("series name is required")

// ErrInvalidDeviceID is returned for a malformed Kobo device ID.
var ErrInvalidDeviceID = errors.New("invalid device ID")

// LibraryService holds the library, progress and reading-state rules that
// span the book, progress, position and Kobo services.
type LibraryService struct {
	logger    *slog.Logger
	books     *BookService
	ingest    *IngestService
	progress  *ProgressService
	positions *positions.PositionService
	physical  *positions.PhysicalService
	kepubs    *conversion.KEPUBQueue
	kobo      *kobo.Service
	koboLog   *kobo.LogStore
}

// StatusUpdate is a request to change a book's status, favourite tag and
// rating. Rating is the raw "1".."5" string; anything else clears it.
type StatusUpdate struct {
	Status    string
	Favourite bool
	Rating    string
}

// ProgressUpdate is a request to record reading progress; ReadAt is when the
// page was read, for syncing a physical copy.
type ProgressUpdate struct {
	Mode    string
	Page    int
	Percent int
	ReadAt  time.Time
}

// ReadingStateView is a stored reading state with its derived forms.
type ReadingStateView struct {
	State           *models.BookReadingState
	Position        models.ReadingPosition
	HasPosition     bool
	PhysicalPage    int
	PhysicalExcerpt string
}

// KEPUBStatusView is a book's file availability and KEPUB conversion status.
type KEPUBStatusView struct {
	HasEPUB     bool
	HasPDF      bool
	KepubStatus string
}

// ParseProgressRange parses an optional date window, defaulting to the last
// year; unparsable values fall back to the default.
func ParseProgressRange(dateStart, dateEnd string) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)
	if t, err := time.Parse(models.ProgressDateFormat, dateStart); err == nil {
		start = t
	}
	if t, err := time.Parse(models.ProgressDateFormat, dateEnd); err == nil {
		end = t
	}
	return start, end
}

// ParseRating returns the 1-5 rating in raw, or nil.
func ParseRating(raw string) *int16 {
	n, err := strconv.ParseInt(raw, 10, 16)
	if err != nil || n <= 0 || n > 5 {
		return nil
	}
	r := int16(n)
	return &r
}

func toggleTag(tags []string, tag string, enable bool) []string {
	result := make([]string, 0, len(tags)+1)
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

func finishedAtFor(existing *models.UserBook, newStatus string) []time.Time {
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

// RebuildReadProgress recomputes and stores userID's reading-progress graph.
func (s *LibraryService) RebuildReadProgress(
	ctx context.Context,
	userID string,
) error {
	labels, values, err := s.books.BuildReadProgress(ctx, userID)
	if err != nil {
		return err
	}
	return s.progress.Save(ctx, userID, labels, values)
}

// SetStatus applies a status change on top of the book's current tags and
// read dates, and rebuilds the progress graph when the book is now read.
func (s *LibraryService) SetStatus(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	req StatusUpdate,
) error {
	existing, err := s.books.GetUserBook(ctx, userID, bookID)
	if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
		return err
	}
	var tags []string
	if existing != nil {
		tags = existing.Tags
	}
	ub := models.UserBook{ //nolint:exhaustruct //optional fields
		UserID:     userID,
		BookID:     bookID,
		Status:     req.Status,
		Tags:       toggleTag(tags, models.TagFavourite, req.Favourite),
		Rating:     ParseRating(req.Rating),
		FinishedAt: finishedAtFor(existing, req.Status),
	}
	if err = s.books.UpdateStatus(ctx, userID, ub); err != nil {
		return err
	}
	if req.Status == models.StatusRead {
		return s.RebuildReadProgress(ctx, userID)
	}
	return nil
}

// SetFinishedAt overwrites the read-date history and rebuilds the graph.
func (s *LibraryService) SetFinishedAt(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	finishedAt []time.Time,
) error {
	if err := s.books.UpdateFinishedAt(ctx, userID, bookID, finishedAt); err != nil {
		return err
	}
	return s.RebuildReadProgress(ctx, userID)
}

// RemoveBook removes a book from the library and rebuilds the graph.
func (s *LibraryService) RemoveBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	if err := s.books.RemoveFromLibrary(ctx, userID, bookID); err != nil {
		return err
	}
	return s.RebuildReadProgress(ctx, userID)
}

// UpdateProgress records progress and, when pages mode moved the page, syncs
// the physical copy's position (ADR-0031). An unchanged page isn't a new
// physical read: syncing it would move a newer, finer digital position back to
// the page's start.
func (s *LibraryService) UpdateProgress(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	req ProgressUpdate,
) error {
	pageChanged := req.Mode == models.ProgressModePages
	if pageChanged {
		ub, err := s.books.GetUserBook(ctx, userID, bookID)
		pageChanged = err != nil || ub.CurrentPage != req.Page
	}
	err := s.books.UpdateProgress(
		ctx, userID, bookID, req.Mode, req.Page, req.Percent,
	)
	if err == nil && pageChanged {
		err = s.physical.SyncPage(ctx, userID, bookID, req.Page, req.ReadAt)
	}
	return err
}

// BookContent returns the stored article HTML, lazily backfilling it when the
// book has none.
func (s *LibraryService) BookContent(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (string, error) {
	html, err := s.books.GetContentHTML(ctx, userID, bookID)
	if err != nil {
		return "", err
	}
	if html != "" {
		return html, nil
	}
	// Ownership was already confirmed above, so a failure here is unexpected
	// rather than a legitimate not-found.
	ub, err := s.books.GetUserBook(ctx, userID, bookID)
	if err != nil {
		return "", err
	}
	return s.ingest.BackfillContentHTML(ctx, ub.Book), nil
}

// ReadingState returns the book's reading state with its reader position and
// physical-copy page, or nil when it has none yet.
func (s *LibraryService) ReadingState(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*ReadingStateView, error) {
	state, err := s.books.GetReadingState(ctx, userID, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil, nil //nolint:nilnil // no state yet is not an error
	}
	if err != nil {
		return nil, err
	}
	view := &ReadingStateView{
		State:           state,
		Position:        models.ReadingPosition{Href: "", Offset: 0, Page: 0},
		HasPosition:     false,
		PhysicalPage:    0,
		PhysicalExcerpt: "",
	}
	if state.Position != nil {
		view.HasPosition = true
		view.PhysicalPage, view.PhysicalExcerpt = s.physical.Locate(
			ctx, userID, bookID, *state.Position,
		)
		view.Position = s.positions.ReaderPosition(ctx, userID, bookID, *state.Position)
	}
	return view, nil
}

// TranslatePosition is the reader position and physical page for pos.
func (s *LibraryService) TranslatePosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	pos models.ReadingPosition,
) (models.ReadingPosition, int) {
	page, _ := s.physical.Locate(ctx, userID, bookID, pos)
	return s.positions.ReaderPosition(ctx, userID, bookID, pos), page
}

// KEPUBStatus reports a book's files and KEPUB status. A stale KEPUB is never
// reported ready: it is reconverted so "converting" resolves.
func (s *LibraryService) KEPUBStatus(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (KEPUBStatusView, error) {
	result, err := s.books.GetKEPUBStatus(ctx, userID, bookID)
	if err != nil {
		return KEPUBStatusView{}, err
	}
	status := result.KepubStatus
	if result.KepubStale && (result.HasEPUB || result.HasPDF) {
		status = s.queueKEPUB(ctx, userID, bookID, status)
	}
	return KEPUBStatusView{
		HasEPUB:     result.HasEPUB,
		HasPDF:      result.HasPDF,
		KepubStatus: status,
	}, nil
}

// RequestKEPUB queues a conversion when the KEPUB is missing or stale and a
// source exists, returning the resulting status.
func (s *LibraryService) RequestKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (string, error) {
	result, err := s.books.GetKEPUBStatus(ctx, userID, bookID)
	if err != nil {
		return "", err
	}
	status := result.KepubStatus
	hasSource := result.HasEPUB || result.HasPDF
	if (status != "" && !result.KepubStale) || !hasSource {
		return status, nil
	}
	return s.queueKEPUB(ctx, userID, bookID, status), nil
}

// EnableKoboSync turns Kobo sync on for a book and queues its KEPUB.
func (s *LibraryService) EnableKoboSync(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (string, error) {
	if err := s.books.EnableKoboSync(ctx, userID, bookID); err != nil {
		return "", err
	}
	return s.RequestKEPUB(ctx, userID, bookID)
}

// queueKEPUB queues a conversion for the workers role: the request must never
// wait on a PDF conversion (ADR-0017). It returns FileStatusConverting, or
// current when queueing failed.
func (s *LibraryService) queueKEPUB(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	current string,
) string {
	if err := s.kepubs.Request(ctx, userID, bookID); err != nil {
		s.logger.ErrorContext(ctx, "queue kepub conversion",
			"book_id", bookID, "err", err)
		return current
	}
	return models.FileStatusConverting
}

// KoboDevices lists the user's devices with their in-memory debug-logging
// state applied.
func (s *LibraryService) KoboDevices(
	ctx context.Context,
	userID string,
) ([]models.KoboDevice, error) {
	devices, err := s.kobo.ListKoboDevices(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range devices {
		devices[i].LoggingEnabled = s.koboLog.IsEnabled(devices[i].ID)
	}
	return devices, nil
}

// KoboLastSeen is the most recent sync across the user's devices, or nil.
func (s *LibraryService) KoboLastSeen(
	ctx context.Context,
	userID string,
) (*time.Time, error) {
	return s.kobo.LastSeenAt(ctx, userID)
}

// KoboDeviceLogs returns a device's captured logs once it is confirmed to
// belong to userID.
func (s *LibraryService) KoboDeviceLogs(
	ctx context.Context,
	userID string,
	deviceID uuid.UUID,
) ([]kobo.LogEntry, error) {
	if _, err := s.kobo.GetKoboDevice(ctx, userID, deviceID); err != nil {
		return nil, err
	}
	return s.koboLog.List(deviceID.String()), nil
}
