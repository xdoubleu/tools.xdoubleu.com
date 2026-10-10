package hardcover

import (
	"context"
	"log/slog"
)

// bookSeriesQuery is GetByISBN's series lookup: editions→book→book_series→
// series would exceed the depth limit of 3.
const bookSeriesQuery = `query BookSeries($id: Int!) {
  book_series(where: {book_id: {_eq: $id}}) {
    position
    series { name primary_books_count }
  }
}`

// seriesQuery picks the most complete unmerged series with the exact name.
const seriesQuery = `query SeriesByName($name: String!) {
  series(
    where: {name: {_eq: $name}, canonical_id: {_is_null: true}}
    order_by: {books_count: desc}
    limit: 1
  ) {
    id
    name
    primary_books_count
  }
}`

// seriesBooksQuery follows Hardcover's "books in a series" guide: one book per
// position, most-read first, skipping compilations and merged books. Rooted at
// book_series so the edition ISBN stays within the depth limit.
const seriesBooksQuery = `query SeriesBooks($id: Int!) {
  book_series(
    distinct_on: position
    order_by: [{position: asc}, {book: {users_count: desc}}]
    where: {
      series_id: {_eq: $id}
      compilation: {_eq: false}
      position: {_is_null: false}
      book: {canonical_id: {_is_null: true}}
    }
  ) {
    position
    book {
      id
      title
      pages
      description
      cached_image
      cached_contributors
      editions(limit: 1, where: {isbn_13: {_is_null: false}}) {
        isbn_13
      }
    }
  }
}`

// GetSeries returns the series' volumes in position order.
func (c client) GetSeries(ctx context.Context, name string) (*Series, error) {
	var resp seriesResponse
	err := c.post(ctx, seriesQuery, map[string]any{"name": name}, &resp)
	if err != nil {
		return nil, err
	}
	if err = graphQLErr(resp.Errors); err != nil {
		return nil, err
	}
	if len(resp.Data.Series) == 0 {
		return nil, ErrNotFound
	}
	rec := resp.Data.Series[0]

	var books bookSeriesResponse
	err = c.post(ctx, seriesBooksQuery, map[string]any{"id": rec.ID}, &books)
	if err != nil {
		return nil, err
	}
	if err = graphQLErr(books.Errors); err != nil {
		return nil, err
	}

	out := &Series{
		Name:  rec.Name,
		Total: positiveOrNil(rec.PrimaryBooksCount),
		Books: make([]SeriesBook, 0, len(books.Data.BookSeries)),
	}
	for _, bs := range books.Data.BookSeries {
		if bs.Book == nil || bs.Position == nil {
			continue
		}
		out.Books = append(out.Books, SeriesBook{
			Position: *bs.Position,
			Book:     bookToExternalBook(*bs.Book),
		})
	}
	return out, nil
}

// bookSeries looks up a book's series. Series data is optional, so a failure
// is logged rather than failing the lookup.
func (c client) bookSeries(ctx context.Context, bookID int) *SeriesRef {
	var resp bookSeriesResponse
	err := c.post(ctx, bookSeriesQuery, map[string]any{"id": bookID}, &resp)
	if err == nil {
		err = graphQLErr(resp.Errors)
	}
	if err != nil {
		c.logger.WarnContext(ctx, "hardcover series lookup failed",
			slog.Int("bookID", bookID), slog.Any("error", err))
		return nil
	}
	return pickSeries(resp.Data.BookSeries)
}

// pickSeries chooses one series for a book in several: ordered entries
// first, then the series with the most main volumes.
func pickSeries(entries []bookSeries) *SeriesRef {
	var best *bookSeries
	for i := range entries {
		e := &entries[i]
		if e.Series == nil || e.Series.Name == "" {
			continue
		}
		if best == nil || betterSeries(*e, *best) {
			best = e
		}
	}
	if best == nil {
		return nil
	}
	return &SeriesRef{
		Name:     best.Series.Name,
		Position: best.Position,
		Total:    positiveOrNil(best.Series.PrimaryBooksCount),
	}
}

func betterSeries(a, b bookSeries) bool {
	if (a.Position != nil) != (b.Position != nil) {
		return a.Position != nil
	}
	return derefInt(a.Series.PrimaryBooksCount) > derefInt(b.Series.PrimaryBooksCount)
}

func positiveOrNil(n *int) *int {
	if n == nil || *n <= 0 {
		return nil
	}
	return n
}

func derefInt(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}
