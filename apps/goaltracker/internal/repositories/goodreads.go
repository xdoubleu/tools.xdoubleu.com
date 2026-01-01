package repositories

import (
	"context"

	"github.com/XDoubleU/essentia/pkg/database/postgres"
	"github.com/jackc/pgx/v5"
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
)

type GoodreadsRepository struct {
	db postgres.DB
}

func (repo *GoodreadsRepository) GetAllBooks(
	ctx context.Context,
	userID string,
) ([]goodreads.Book, error) {
	query := `
		SELECT id, shelf, tags, title, author, dates_read
		FROM goaltracker.goodreads_books
		WHERE user_id = $1 
	`

	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	books := []goodreads.Book{}
	for rows.Next() {
		var book goodreads.Book

		err = rows.Scan(
			&book.ID,
			&book.Shelf,
			&book.Tags,
			&book.Title,
			&book.Author,
			&book.DatesRead,
		)

		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		books = append(books, book)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return books, nil
}

func (repo *GoodreadsRepository) GetAllTags(
	ctx context.Context,
	userID string,
) ([]string, error) {
	query := `
		SELECT ARRAY_AGG(DISTINCT tag) AS tags 
		FROM 
			goaltracker.goodreads_books,
			UNNEST(tags) as tag
		WHERE tags <> '{}' AND user_id = $1;
	`

	tags := []string{}
	err := repo.db.QueryRow(ctx, query, userID).Scan(&tags)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return tags, nil
}

func (repo *GoodreadsRepository) GetBooksByTag(
	ctx context.Context,
	tag string,
	userID string,
) ([]goodreads.Book, error) {
	query := `
		SELECT id, shelf, tags, title, author, dates_read
		FROM goaltracker.goodreads_books 
		WHERE $1 = ANY(tags) AND user_id = $2
	`

	rows, err := repo.db.Query(ctx, query, tag, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	books := []goodreads.Book{}
	for rows.Next() {
		var book goodreads.Book

		err = rows.Scan(
			&book.ID,
			&book.Shelf,
			&book.Tags,
			&book.Title,
			&book.Author,
			&book.DatesRead,
		)

		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		books = append(books, book)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return books, nil
}

func (repo *GoodreadsRepository) GetBooksByIDs(
	ctx context.Context,
	ids []int64,
	userID string,
) ([]goodreads.Book, error) {
	query := `
		SELECT id, shelf, tags, title, author, dates_read
		FROM goaltracker.goodreads_books 
		WHERE id = ANY($1) AND user_id = $2
	`

	rows, err := repo.db.Query(ctx, query, ids, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	books := []goodreads.Book{}
	for rows.Next() {
		var book goodreads.Book

		err = rows.Scan(
			&book.ID,
			&book.Shelf,
			&book.Tags,
			&book.Title,
			&book.Author,
			&book.DatesRead,
		)

		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		books = append(books, book)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return books, nil
}

func (repo *GoodreadsRepository) UpsertBooks(
	ctx context.Context,
	books []goodreads.Book,
	userID string,
) error {
	query := `
		INSERT INTO goaltracker.goodreads_books 
		(id, user_id, shelf, tags, title, author, dates_read)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id, user_id)
		DO UPDATE SET shelf = $3, tags = $4, title = $5, author = $6, dates_read = $7
	`

	//nolint:exhaustruct //fields are optional
	b := &pgx.Batch{}
	for _, book := range books {
		b.Queue(
			query,
			book.ID,
			userID,
			book.Shelf,
			book.Tags,
			book.Title,
			book.Author,
			book.DatesRead,
		)
	}

	err := repo.db.SendBatch(ctx, b).Close()
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}
