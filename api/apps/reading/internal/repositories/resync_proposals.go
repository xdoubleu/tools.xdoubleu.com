package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/reading/internal/models"
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
	b.description, b.page_count, b.category, b.source_url,
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
		&book.Category,
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

	if _, err = tx.Exec(ctx, `DELETE FROM reading.resync_proposals`); err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	for bookID, raw := range entries {
		_, err = tx.Exec(ctx, `
			INSERT INTO reading.resync_proposals (book_id, proposals)
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
		FROM reading.resync_proposals rp
		JOIN reading.books b ON b.id = rp.book_id
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
		FROM reading.resync_proposals rp
		JOIN reading.books b ON b.id = rp.book_id
		WHERE rp.book_id = $1
	`

	row := repo.db.QueryRow(ctx, query, bookID)
	book, err := scanResyncProposalRow(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return book, nil
}

// SourceStats aggregates per-source scan coverage, uniqueness, and pairwise
// overlap over the whole catalog, for the admin source-stats report.
//
// Uniqueness and overlap only count a source as "absent" when it was
// actually checked and came back empty (IS FALSE) — a source that's still
// unknown (NULL: never scanned, skipped, or errored — see
// UpdateResyncScanStatus) must never be treated as confirmed-absent, or an
// unresolved source would masquerade as uniqueness/overlap it hasn't earned.
type SourceStats struct {
	TotalBooks     int
	UniCatFound    int
	HardcoverFound int
	// Unique counts books found in this source and confirmed absent
	// (IS FALSE) from the other.
	UniCatUnique    int
	HardcoverUnique int
	// Missed counts a source actually checked and came back empty
	// (found_column IS FALSE) — distinct from never having been scanned.
	UniCatMissed    int
	HardcoverMissed int
	// Both is found in both sources. BothMissed is both sources explicitly
	// checked and came back empty (strict IS FALSE, no NULLs) — stricter than
	// NotFoundAnywhere, which also counts a book with an unresolved (NULL)
	// source as long as neither confirmed found.
	Both             int
	BothMissed       int
	NotFoundAnywhere int
	NeverScanned     int
}

// sourceStatsQuery computes every SourceStats aggregate over the two-source
// partition (found/missed/unique per source, the "found by both" combo, and
// the catalog totals) in one pass.
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
		FROM reading.books
		WHERE category = 'book'
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

// DeleteResyncProposal removes one book's stored proposal, e.g. after the
// admin applies or dismisses it. A no-op if none exists.
func (repo *BooksRepository) DeleteResyncProposal(
	ctx context.Context,
	bookID uuid.UUID,
) error {
	_, err := repo.db.Exec(
		ctx, `DELETE FROM reading.resync_proposals WHERE book_id = $1`, bookID,
	)
	return postgres.PgxErrorToHTTPError(err)
}
