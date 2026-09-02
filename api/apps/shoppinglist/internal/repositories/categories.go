package repositories

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database/postgres"
)

type Category struct {
	ID   string
	Name string
}

func (r *ShoppingRepository) ListCategories(
	ctx context.Context,
	familyID uuid.UUID,
) ([]Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name
		FROM shoppinglist.categories
		WHERE family_id = $1
		ORDER BY name`,
		familyID,
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
	familyID uuid.UUID,
	name string,
) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		INSERT INTO shoppinglist.categories (family_id, name)
		VALUES ($1, $2)
		RETURNING id::text, name`,
		familyID, name,
	).Scan(&c.ID, &c.Name)
	if err != nil {
		return Category{}, postgres.PgxErrorToHTTPError(err)
	}
	return c, nil
}

func (r *ShoppingRepository) RenameCategory(
	ctx context.Context,
	familyID uuid.UUID,
	id uuid.UUID,
	name string,
) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		UPDATE shoppinglist.categories
		SET name = $3
		WHERE id = $1 AND family_id = $2
		RETURNING id::text, name`,
		id, familyID, name,
	).Scan(&c.ID, &c.Name)
	if err != nil {
		return Category{}, postgres.PgxErrorToHTTPError(err)
	}
	return c, nil
}

func (r *ShoppingRepository) DeleteCategory(
	ctx context.Context,
	familyID uuid.UUID,
	id uuid.UUID,
) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM shoppinglist.categories
		WHERE id = $1 AND family_id = $2`,
		id, familyID,
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
