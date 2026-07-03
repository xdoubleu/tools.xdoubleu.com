package repositories

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/progresshistory"
)

type ProgressRepository struct {
	db postgres.DB
}

func (repo *ProgressRepository) GetByDates(
	ctx context.Context,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]progresshistory.Record, error) {
	query := `
		SELECT value, date
		FROM books.progress
		WHERE user_id = $1 AND date >= $2 AND date <= $3
		ORDER BY date ASC
	`

	rows, err := repo.db.Query(ctx, query, userID, dateStart, dateEnd)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	records := []progresshistory.Record{}
	for rows.Next() {
		var record progresshistory.Record

		err = rows.Scan(&record.Value, &record.Date)
		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return records, nil
}

func (repo *ProgressRepository) GetLastValueBefore(
	ctx context.Context,
	userID string,
	date time.Time,
) (string, error) {
	query := `
		SELECT value
		FROM books.progress
		WHERE user_id = $1 AND date < $2::date
		ORDER BY date DESC
		LIMIT 1
	`

	var value string
	err := repo.db.QueryRow(ctx, query, userID, date).Scan(&value)
	if err != nil {
		return "", postgres.PgxErrorToHTTPError(err)
	}

	return value, nil
}

func (repo *ProgressRepository) Upsert(
	ctx context.Context,
	userID string,
	dates []string,
	values []string,
) error {
	query := `
		INSERT INTO books.progress (user_id, date, value)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, date)
		DO UPDATE SET value = $3
	`

	//nolint:exhaustruct //fields are optional
	b := &pgx.Batch{}
	for i := range dates {
		date, _ := time.Parse(models.ProgressDateFormat, dates[i])
		b.Queue(query, userID, date, values[i])
	}

	err := repo.db.SendBatch(ctx, b).Close()
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}
