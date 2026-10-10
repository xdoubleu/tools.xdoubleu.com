package services

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
)

// seriesLookupTimeout keeps a slow Hardcover well inside the proxy's response
// timeout, so GetSeries degrades to the library instead of being reset.
const seriesLookupTimeout = 10 * time.Second

// SeriesEntry is one volume: the user's own book, or a Hardcover volume they
// don't have yet (External).
type SeriesEntry struct {
	Position *float64
	UserBook *models.UserBook
	External *SourceProposal
}

// SeriesView is a series as the user sees it, in position order.
type SeriesView struct {
	Name    string
	Total   *int
	Entries []SeriesEntry
	// ExternalUnavailable means Hardcover couldn't be asked, so missing
	// volumes are unknown rather than absent.
	ExternalUnavailable bool
}

// GetSeries merges the user's books in the named series with Hardcover's
// volumes. A Hardcover volume matches a library book by title, else by
// position. A Hardcover failure degrades to the library alone.
func (s *BookService) GetSeries(
	ctx context.Context,
	userID string,
	name string,
) (*SeriesView, error) {
	owned, err := s.books.ListUserBooksInSeries(ctx, userID, name)
	if err != nil {
		return nil, err
	}

	view := &SeriesView{Name: name} //nolint:exhaustruct // filled below
	for i := range owned {
		if t := owned[i].Book.Series.Total; t != nil {
			view.Total = t
		}
	}

	hc, ok := s.fetchHardcoverSeries(ctx, name)
	view.ExternalUnavailable = !ok
	if hc != nil && hc.Total != nil {
		view.Total = hc.Total
		if totalErr := s.books.SetSeriesTotal(ctx, name, *hc.Total); totalErr != nil {
			s.logger.WarnContext(ctx, "failed to cache series total",
				"series", name, "error", totalErr)
		}
	}

	view.Entries = mergeSeriesEntries(name, owned, hc)
	return view, nil
}

// fetchHardcoverSeries reports ok=false when Hardcover is unconfigured or
// failed; a series Hardcover doesn't know is (nil, true).
func (s *BookService) fetchHardcoverSeries(
	ctx context.Context,
	name string,
) (*hardcover.Series, bool) {
	if s.hardcover == nil {
		return nil, false
	}
	if hc, hit := s.seriesCache.get(name); hit {
		return hc, true
	}

	ctx, cancel := context.WithTimeout(ctx, seriesLookupTimeout)
	defer cancel()
	hc, err := s.hardcover.GetSeries(ctx, name)
	if errors.Is(err, hardcover.ErrNotFound) {
		s.seriesCache.put(name, nil)
		return nil, true
	}
	if err != nil {
		s.logger.WarnContext(ctx, "hardcover series lookup failed",
			"series", name, "error", err)
		return nil, false
	}
	s.seriesCache.put(name, hc)
	return hc, true
}

func mergeSeriesEntries(
	name string,
	owned []models.UserBook,
	hc *hardcover.Series,
) []SeriesEntry {
	var volumes []hardcover.SeriesBook
	if hc != nil {
		volumes = hc.Books
	}
	match := matchOwnedVolumes(owned, volumes)

	var entries []SeriesEntry
	used := make([]bool, len(owned))
	for v, vol := range volumes {
		pos := vol.Position
		if i := match[v]; i >= 0 {
			used[i] = true
			entries = append(entries, SeriesEntry{
				Position: positionOr(owned[i].Book.Series.Position, &pos),
				UserBook: &owned[i],
				External: nil,
			})
			continue
		}
		ext := newSourceProposalFromCandidate("hardcover", hcCandidate(vol.Book))
		ext.SeriesName = name
		ext.SeriesPosition = &pos
		ext.SeriesTotal = hc.Total
		entries = append(entries, SeriesEntry{
			Position: &pos, UserBook: nil, External: &ext,
		})
	}

	for i := range owned {
		if !used[i] {
			entries = append(entries, SeriesEntry{
				Position: owned[i].Book.Series.Position,
				UserBook: &owned[i],
				External: nil,
			})
		}
	}

	slices.SortStableFunc(entries, func(a, b SeriesEntry) int {
		if a.Position == nil || b.Position == nil {
			// Unpositioned entries sort last.
			return boolRank(a.Position == nil) - boolRank(b.Position == nil)
		}
		return cmp.Compare(*a.Position, *b.Position)
	})
	return entries
}

// matchOwnedVolumes maps each volume to the owned book it is (-1 for none):
// every title match first, then positions among what's left, so a mistyped
// position can't steal a volume another book matches by title.
func matchOwnedVolumes(owned []models.UserBook, volumes []hardcover.SeriesBook) []int {
	match := make([]int, len(volumes))
	used := make([]bool, len(owned))
	for v, vol := range volumes {
		match[v] = -1
		title := normalizeTitle(vol.Book.Title)
		if title == "" {
			continue
		}
		for i, ub := range owned {
			if !used[i] && normalizeTitle(ub.Book.Title) == title {
				match[v], used[i] = i, true
				break
			}
		}
	}
	for v, vol := range volumes {
		if match[v] >= 0 {
			continue
		}
		for i, ub := range owned {
			p := ub.Book.Series.Position
			if !used[i] && p != nil && *p == vol.Position {
				match[v], used[i] = i, true
				break
			}
		}
	}
	return match
}

func positionOr(p, fallback *float64) *float64 {
	if p != nil {
		return p
	}
	return fallback
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// mergeSeries is what a merge does with series: the winner's own, or the
// first loser's when the winner has none (inherited).
type mergeSeries struct {
	series    *models.BookSeries
	inherited bool
}

// seriesForMerge is best-effort: a failed lookup only loses the inheritance.
func (s *BookService) seriesForMerge(
	ctx context.Context,
	winnerID uuid.UUID,
	loserIDs []uuid.UUID,
) mergeSeries {
	books, err := s.books.GetBooksByIDs(ctx, append([]uuid.UUID{winnerID}, loserIDs...))
	if err != nil {
		s.logger.WarnContext(ctx, "failed to load series for merge",
			"bookID", winnerID, "error", err)
		return mergeSeries{} //nolint:exhaustruct // nothing to keep
	}
	byID := make(map[uuid.UUID]*models.BookSeries, len(books))
	for _, b := range books {
		byID[b.ID] = b.Series
	}
	if own := byID[winnerID]; own != nil {
		return mergeSeries{series: own, inherited: false}
	}
	for _, id := range loserIDs {
		if series := byID[id]; series != nil {
			return mergeSeries{series: series, inherited: true}
		}
	}
	return mergeSeries{} //nolint:exhaustruct // no entry has a series
}

// applyMergedMetadata writes a merge's resolved metadata, keeping the merged
// series unless the resolution names its own; without a resolution it only
// writes an inherited series, best-effort like seriesForMerge.
func (s *BookService) applyMergedMetadata(
	ctx context.Context,
	winnerID uuid.UUID,
	resolved *models.Book,
	ms mergeSeries,
) error {
	if resolved == nil {
		if !ms.inherited {
			return nil
		}
		if err := s.books.SetBookSeries(ctx, winnerID, ms.series); err != nil {
			s.logger.WarnContext(ctx, "failed to apply merged series",
				"bookID", winnerID, "error", err)
		}
		return nil
	}
	resolved.ID = winnerID
	if resolved.Series == nil {
		resolved.Series = ms.series
	}
	if err := s.books.UpdateBookByID(ctx, *resolved); err != nil {
		return fmt.Errorf("apply resolved metadata: %w", err)
	}
	return nil
}
