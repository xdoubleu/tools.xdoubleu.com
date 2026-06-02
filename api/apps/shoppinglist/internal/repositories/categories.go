package repositories

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	iapp "tools.xdoubleu.com/internal/app"
)

type Category struct {
	ID   string
	Name string
}

func (r *ShoppingRepository) ListCategories(
	ctx context.Context,
	userID string,
) ([]Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name
		FROM shoppinglist.categories
		WHERE user_id = $1
		ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Category
	for rows.Next() {
		var c Category
		if err = rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *ShoppingRepository) CreateCategory(
	ctx context.Context,
	userID, name string,
) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		INSERT INTO shoppinglist.categories (user_id, name)
		VALUES ($1, $2)
		RETURNING id::text, name`,
		userID, name,
	).Scan(&c.ID, &c.Name)
	if err != nil {
		return Category{}, postgres.PgxErrorToHTTPError(err)
	}
	return c, nil
}

func (r *ShoppingRepository) RenameCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		UPDATE shoppinglist.categories
		SET name = $3
		WHERE id = $1 AND user_id = $2
		RETURNING id::text, name`,
		id, userID, name,
	).Scan(&c.ID, &c.Name)
	if err != nil {
		return Category{}, postgres.PgxErrorToHTTPError(err)
	}
	return c, nil
}

func (r *ShoppingRepository) DeleteCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM shoppinglist.categories
		WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Category not found",
		}
	}
	return nil
}
