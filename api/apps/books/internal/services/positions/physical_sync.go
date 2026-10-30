package positions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/threading"
)

// Limits on a physical copy's details.
const (
	maxPhysicalPages   = 100_000
	maxPhysicalAnchors = 50
)

// ErrInvalidPhysicalCopy rejects details out of range.
var ErrInvalidPhysicalCopy = errors.New("invalid physical copy")

type physicalCopyStore interface {
	GetPhysicalCopy(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*models.PhysicalCopy, error)
	UpdatePhysicalCopy(
		ctx context.Context, userID string, bookID uuid.UUID, c models.PhysicalCopy,
	) error
	SetCurrentPageIfLatest(
		ctx context.Context, userID string, bookID uuid.UUID, page int, at time.Time,
	) error
}

type physicalStateStore interface {
	UpsertIfNewer(ctx context.Context, state models.BookReadingState) (*time.Time, error)
	SetPhysicalPosition(
		ctx context.Context, userID string, bookID uuid.UUID, updatedAt time.Time,
		pos models.ReadingPosition, percent int,
	) error
}

// PhysicalService keeps a physical copy's page and the digital reading
// position in step for a book owned both ways (ADR-0031).
type PhysicalService struct {
	logger       *slog.Logger
	background   *threading.Background
	copies       physicalCopyStore
	states       physicalStateStore
	positions    *PositionService
	budget       time.Duration
	buildTimeout time.Duration
}

func NewPhysicalService(
	logger *slog.Logger,
	background *threading.Background,
	copies physicalCopyStore,
	states physicalStateStore,
	positions *PositionService,
) *PhysicalService {
	return &PhysicalService{
		logger:       logger,
		background:   background,
		copies:       copies,
		states:       states,
		positions:    positions,
		budget:       positions.budget,
		buildTimeout: positions.buildTimeout,
	}
}

// PhysicalChapter is a TOC entry offered as an anchor.
type PhysicalChapter struct {
	Title    string
	Position models.ReadingPosition
}

// PhysicalCopyView is a copy's details plus what its EPUB offers for them.
type PhysicalCopyView struct {
	Copy models.PhysicalCopy
	// Syncable also needs an EPUB to map through.
	Syncable      bool
	HasPrintPages bool
	Chapters      []PhysicalChapter
}

// Copy returns the user's physical copy of bookID. Chapters and print pages
// are left out when the EPUB isn't parsed within the budget.
func (s *PhysicalService) Copy(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*PhysicalCopyView, error) {
	c, err := s.copies.GetPhysicalCopy(ctx, userID, bookID)
	if err != nil {
		return nil, err
	}
	view := &PhysicalCopyView{
		Copy: *c, Syncable: false, HasPrintPages: false, Chapters: nil,
	}
	if !c.Syncable {
		return view, nil
	}
	t, pending := s.positions.bookText(ctx, userID, bookID, s.budget)
	view.Syncable = t != nil || pending
	if t == nil {
		return view, nil
	}
	view.HasPrintPages = t.hasPrintPages()
	for _, ch := range t.chapters {
		view.Chapters = append(view.Chapters, PhysicalChapter{
			Title: ch.title, Position: t.position(ch.at),
		})
	}
	return view, nil
}

// UpdateCopy stores the copy's details.
func (s *PhysicalService) UpdateCopy(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	c models.PhysicalCopy,
) error {
	if c.PageCount < 0 || c.PageCount > maxPhysicalPages ||
		c.TextStartPage < 0 || c.TextStartPage > maxPhysicalPages ||
		len(c.Anchors) > maxPhysicalAnchors {
		return ErrInvalidPhysicalCopy
	}
	for _, a := range c.Anchors {
		if a.Page <= 0 || a.Page > maxPhysicalPages || a.Href == "" || a.Offset < 0 {
			return fmt.Errorf("%w: anchor needs a page, href and offset >= 0",
				ErrInvalidPhysicalCopy)
		}
	}
	return s.copies.UpdatePhysicalCopy(ctx, userID, bookID, c)
}

// SyncPage writes a page entered for the physical copy as a physical reading
// state read at readAt, so the Kobo and web reader follow it. When the EPUB
// isn't parsed within the budget, its position is filled in later.
func (s *PhysicalService) SyncPage(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	page int,
	readAt time.Time,
) error {
	if page <= 0 {
		return nil
	}
	c, err := s.copies.GetPhysicalCopy(ctx, userID, bookID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return nil
	}
	if err != nil || !c.Syncable {
		return err
	}
	state := func(pos *models.ReadingPosition, percent int) models.BookReadingState {
		return models.BookReadingState{
			UserID: userID, BookID: bookID, Source: models.ReadingSourcePhysical,
			Percent: percent, Location: nil, KoboLocation: nil,
			Position: pos, ReadAt: &readAt, UpdatedAt: time.Time{},
		}
	}

	t, pending := s.positions.bookText(ctx, userID, bookID, s.budget)
	if t != nil {
		pos, g, ok := t.positionOfPage(*c, page)
		if !ok {
			return nil
		}
		_, err = s.states.UpsertIfNewer(ctx, state(&pos, t.percentAt(g)))
		return err
	}
	if !pending || c.PageCount <= 0 {
		return nil
	}
	// Stored now, its position filled in once the EPUB is parsed, so a digital
	// write waiting on the same parse can't move current_page off this page.
	updatedAt, err := s.states.UpsertIfNewer(ctx,
		state(nil, min(page*models.MaxProgressPercent/c.PageCount,
			models.MaxProgressPercent)))
	if err != nil || updatedAt == nil {
		return err
	}
	cp, at := *c, *updatedAt
	s.background.Go(ctx, "physical-position-fill", func(ctx context.Context) {
		s.fillPhysicalPosition(ctx, userID, bookID, cp, page, at)
	})
	return nil
}

// fillPhysicalPosition sets the position of the physical state stored at
// updatedAt once the EPUB is parsed.
func (s *PhysicalService) fillPhysicalPosition(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	c models.PhysicalCopy,
	page int,
	updatedAt time.Time,
) {
	t, _ := s.positions.bookText(ctx, userID, bookID, s.buildTimeout)
	if t == nil {
		return
	}
	pos, g, ok := t.positionOfPage(c, page)
	if !ok {
		return
	}
	if err := s.states.SetPhysicalPosition(
		ctx, userID, bookID, updatedAt, pos, t.percentAt(g),
	); err != nil {
		s.logger.WarnContext(ctx, "physical position backfill failed",
			"book_id", bookID, "err", err)
	}
}

// FollowDigital sets current_page, in the background, from a web or Kobo
// state stored at updatedAt, unless a newer write lands first.
func (s *PhysicalService) FollowDigital(
	ctx context.Context,
	state models.BookReadingState,
	updatedAt time.Time,
) {
	if state.Source == models.ReadingSourcePhysical || state.Position == nil ||
		state.Position.Page > 0 {
		return
	}
	s.background.Go(ctx, "physical-follow-digital", func(ctx context.Context) {
		s.followDigital(ctx, state, updatedAt)
	})
}

func (s *PhysicalService) followDigital(
	ctx context.Context,
	state models.BookReadingState,
	updatedAt time.Time,
) {
	c, err := s.copies.GetPhysicalCopy(ctx, state.UserID, state.BookID)
	if err != nil || !c.Syncable {
		return
	}
	t, _ := s.positions.bookText(ctx, state.UserID, state.BookID, s.buildTimeout)
	if t == nil {
		return
	}
	page := t.pageOf(*c, *state.Position)
	if page <= 0 {
		return
	}
	if err = s.copies.SetCurrentPageIfLatest(
		ctx, state.UserID, state.BookID, page, updatedAt,
	); err != nil {
		s.logger.WarnContext(ctx, "following a digital position failed",
			"book_id", state.BookID, "err", err)
	}
}

// Locate is the physical page at pos and the sentence there, or 0 and "" when
// the book isn't owned both ways or the EPUB isn't parsed within the budget.
func (s *PhysicalService) Locate(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	pos models.ReadingPosition,
) (int, string) {
	if pos.Page > 0 {
		return 0, ""
	}
	c, err := s.copies.GetPhysicalCopy(ctx, userID, bookID)
	if err != nil || !c.Syncable {
		return 0, ""
	}
	t, _ := s.positions.bookText(ctx, userID, bookID, s.budget)
	if t == nil {
		return 0, ""
	}
	page := t.pageOf(*c, pos)
	if page <= 0 {
		return 0, ""
	}
	return page, t.excerpt(pos)
}
