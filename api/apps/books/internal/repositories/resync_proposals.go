package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

// ResyncProposalRow pairs a catalog book with its stored proposals JSON,
// passed through opaquely.
type ResyncProposalRow struct {
	Book          models.Book
	ProposalsJSON []byte
}

// resyncProposalColumns is bookColumns qualified with "b." plus the proposals blob.
const resyncProposalColumns = `b.id, b.title, b.authors, b.isbn13, b.cover_url,
	b.description, b.page_count, b.source_url,
	b.created_at, b.updated_at,
	b.unicat_found, b.hardcover_found,
	b.last_resync_at, b.metadata_source,
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
		&book.SourceURL,
		&book.CreatedAt,
		&book.UpdatedAt,
		&book.UniCatFound,
		&book.HardcoverFound,
		&book.LastResyncAt,
		&book.MetadataSource,
		&out.ProposalsJSON,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ReplaceResyncProposals atomically replaces the whole table, so books no
// longer flagged drop out.
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

// ListResyncProposals returns every stored proposal with its book, by title.
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

// GetResyncProposal returns one book's proposal, or ErrResourceNotFound.
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

// SourceStats aggregates per-source coverage, uniqueness and overlap. A
// source counts as absent only when checked and empty (IS FALSE), never
// when unresolved (NULL).
type SourceStats struct {
	TotalBooks     int
	UniCatFound    int
	HardcoverFound int
	// Unique: found here and confirmed absent from the other.
	UniCatUnique    int
	HardcoverUnique int
	// Missed: checked and empty, distinct from never scanned.
	UniCatMissed    int
	HardcoverMissed int
	// BothMissed requires both explicitly empty (no NULLs), unlike
	// NotFoundAnywhere.
	Both             int
	BothMissed       int
	NotFoundAnywhere int
	NeverScanned     int
}

const sourceStatsQuery = `
		SELECT count(*),
		    count(*) FILTER (WHERE unicat_found),
		    count(*) FILTER (WHERE hardcover_found),
		    count(*) FILTER (WHERE unicat_found IS FALSE),
		    count(*) FILTER (WHERE hardcover_found IS FALSE),
		    count(*) FILTER (WHERE unicat_found IS TRUE
		        AND hardcover_found IS FALSE),
		    count(*) FILTER (WHERE hardcover_found IS TRUE
		        AND unicat_found IS FALSE),
		    count(*) FILTER (WHERE unicat_found IS TRUE
		        AND hardcover_found IS TRUE),
		    count(*) FILTER (WHERE unicat_found IS FALSE
		        AND hardcover_found IS FALSE),
		    count(*) FILTER (WHERE last_resync_at IS NOT NULL
		        AND NOT COALESCE(unicat_found, false)
		        AND NOT COALESCE(hardcover_found, false)),
		    count(*) FILTER (WHERE last_resync_at IS NULL)
		FROM books.books
		WHERE source_url IS NULL
	`

// GetSourceStats computes SourceStats in a single aggregate query.
func (repo *BooksRepository) GetSourceStats(
	ctx context.Context,
) (*SourceStats, error) {
	var stats SourceStats
	err := repo.db.QueryRow(ctx, sourceStatsQuery).Scan(
		&stats.TotalBooks,
		&stats.UniCatFound,
		&stats.HardcoverFound,
		&stats.UniCatMissed,
		&stats.HardcoverMissed,
		&stats.UniCatUnique,
		&stats.HardcoverUnique,
		&stats.Both,
		&stats.BothMissed,
		&stats.NotFoundAnywhere,
		&stats.NeverScanned,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &stats, nil
}

// DeleteResyncProposal removes one book's proposal; a no-op if none exists.
func (repo *BooksRepository) DeleteResyncProposal(
	ctx context.Context,
	bookID uuid.UUID,
) error {
	_, err := repo.db.Exec(
		ctx, `DELETE FROM books.resync_proposals WHERE book_id = $1`, bookID,
	)
	return postgres.PgxErrorToHTTPError(err)
}
