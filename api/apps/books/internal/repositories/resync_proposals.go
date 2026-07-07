package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// ResyncProposalRow pairs a catalog book with its stored resync proposals
// (JSON-encoded []services.SourceProposal — this package stays agnostic of
// that type and passes the bytes through).
type ResyncProposalRow struct {
	Book          models.Book
	ProposalsJSON []byte
}

// resyncProposalColumns mirrors bookColumns but qualified with the "b." alias
// (bookColumns is unqualified and only safe against a single unaliased
// table — see GetCatalogWithUserOverlay for the same pattern) joined with the
// stored proposals blob.
const resyncProposalColumns = `b.id, b.title, b.authors, b.isbn13, b.cover_url,
	b.description, b.page_count, b.created_at, b.updated_at,
	b.openlibrary_found, b.googlebooks_found, b.unicat_found, b.last_resync_at,
	rp.proposals`

func scanResyncProposalRow(row pgx.Row) (*ResyncProposalRow, error) {
	var out ResyncProposalRow
	book := &out.Book

	err := row.Scan(
		&book.ID,
		&book.Title,
		&book.Authors,
		&book.ISBN13,
		&book.CoverURL,
		&book.Description,
		&book.PageCount,
		&book.CreatedAt,
		&book.UpdatedAt,
		&book.OpenLibraryFound,
		&book.GoogleBooksFound,
		&book.UniCatFound,
		&book.LastResyncAt,
		&out.ProposalsJSON,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ReplaceResyncProposals atomically replaces the entire resync_proposals
// table with the given book_id -> proposals-JSON entries. Used by a full
// catalog resync scan: books no longer flagged (agree with every source, or
// no longer exist) are dropped so the admin wizard never shows stale rows.
func (repo *BooksRepository) ReplaceResyncProposals(
	ctx context.Context,
	entries map[uuid.UUID][]byte,
) error {
	//nolint:exhaustruct //default tx options
	tx, err := repo.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `DELETE FROM books.resync_proposals`); err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	for bookID, raw := range entries {
		_, err = tx.Exec(ctx, `
			INSERT INTO books.resync_proposals (book_id, proposals)
			VALUES ($1, $2::jsonb)
		`, bookID, string(raw))
		if err != nil {
			return postgres.PgxErrorToHTTPError(err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	return nil
}

// ListResyncProposals returns every stored proposal joined with its current
// catalog book row, ordered by title.
func (repo *BooksRepository) ListResyncProposals(
	ctx context.Context,
) ([]ResyncProposalRow, error) {
	query := `
		SELECT ` + resyncProposalColumns + `
		FROM books.resync_proposals rp
		JOIN books.books b ON b.id = rp.book_id
		ORDER BY b.title
	`

	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []ResyncProposalRow
	for rows.Next() {
		book, scanErr := scanResyncProposalRow(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, *book)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// GetResyncProposal returns one book's stored proposals joined with its
// current catalog book row. Returns database.ErrResourceNotFound when the
// book has no pending proposal (already applied, dismissed, or never flagged).
func (repo *BooksRepository) GetResyncProposal(
	ctx context.Context,
	bookID uuid.UUID,
) (*ResyncProposalRow, error) {
	query := `
		SELECT ` + resyncProposalColumns + `
		FROM books.resync_proposals rp
		JOIN books.books b ON b.id = rp.book_id
		WHERE rp.book_id = $1
	`

	row := repo.db.QueryRow(ctx, query, bookID)
	book, err := scanResyncProposalRow(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return book, nil
}

// DeleteResyncProposal removes one book's stored proposal, e.g. after the
// admin applies or dismisses it. A no-op if none exists.
func (repo *BooksRepository) DeleteResyncProposal(
	ctx context.Context,
	bookID uuid.UUID,
) error {
	_, err := repo.db.Exec(
		ctx, `DELETE FROM books.resync_proposals WHERE book_id = $1`, bookID,
	)
	return postgres.PgxErrorToHTTPError(err)
}
