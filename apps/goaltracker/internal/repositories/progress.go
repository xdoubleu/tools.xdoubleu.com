package repositories

import (
	"context"
	"time"

	"github.com/XDoubleU/essentia/pkg/database/postgres"
	"github.com/jackc/pgx/v5"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
)

type ProgressRepository struct {
	db postgres.DB
}

func (repo *ProgressRepository) GetByTypeIDAndDates(
	ctx context.Context,
	typeID int64,
	userID string,
	dateStart time.Time,
	dateEnd time.Time,
) ([]models.Progress, error) {
	query := `
		SELECT value, date 
		FROM goaltracker.progress 
		WHERE type_id = $1 AND user_id = $2 AND date >= $3 AND date <= $4
		ORDER BY date ASC
	`

	rows, err := repo.db.Query(ctx, query, typeID, userID, dateStart, dateEnd)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	progresses := []models.Progress{}
	for rows.Next() {
		//nolint:exhaustruct //other fields are assigned later
		progress := models.Progress{
			TypeID: typeID,
		}

		err = rows.Scan(
			&progress.Value,
			&progress.Date,
		)

		if err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}

		progresses = append(progresses, progress)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return progresses, nil
}

func (repo *ProgressRepository) Upsert(
	ctx context.Context,
	typeID int64,
	userID string,
	dates []string,
	values []string,
) error {
	query := `
		INSERT INTO goaltracker.progress (type_id, user_id, date, value)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (type_id, user_id, date)
		DO UPDATE SET value = $4
	`

	//nolint:exhaustruct //fields are optional
	b := &pgx.Batch{}
	for i := range dates {
		date, _ := time.Parse(models.ProgressDateFormat, dates[i])
		b.Queue(query, typeID, userID, date, values[i])
	}

	err := repo.db.SendBatch(ctx, b).Close()
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	return nil
}
