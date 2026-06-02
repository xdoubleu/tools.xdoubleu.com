package repositories

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	iapp "tools.xdoubleu.com/internal/app"
)

type Store struct {
	ID   string
	Name string
}

func (r *ShoppingRepository) ListStores(
	ctx context.Context,
	userID string,
) ([]Store, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, name
		FROM shoppinglist.stores
		WHERE user_id = $1
		ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Store
	for rows.Next() {
		var s Store
		if err = rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *ShoppingRepository) CreateStore(
	ctx context.Context,
	userID, name string,
) (Store, error) {
	var s Store
	err := r.db.QueryRow(ctx, `
		INSERT INTO shoppinglist.stores (user_id, name)
		VALUES ($1, $2)
		RETURNING id::text, name`,
		userID, name,
	).Scan(&s.ID, &s.Name)
	if err != nil {
		return Store{}, postgres.PgxErrorToHTTPError(err)
	}
	return s, nil
}

func (r *ShoppingRepository) RenameStore(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) (Store, error) {
	var s Store
	err := r.db.QueryRow(ctx, `
		UPDATE shoppinglist.stores
		SET name = $3
		WHERE id = $1 AND user_id = $2
		RETURNING id::text, name`,
		id, userID, name,
	).Scan(&s.ID, &s.Name)
	if err != nil {
		return Store{}, postgres.PgxErrorToHTTPError(err)
	}
	return s, nil
}

func (r *ShoppingRepository) DeleteStore(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM shoppinglist.stores
		WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Store not found",
		}
	}
	return nil
}

// GetStoreCategories returns the store's categories in walk-through order.
// It verifies the store belongs to userID.
func (r *ShoppingRepository) GetStoreCategories(
	ctx context.Context,
	userID string,
	storeID uuid.UUID,
) ([]Category, error) {
	if err := r.checkStoreOwnership(ctx, userID, storeID); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT c.id::text, c.name
		FROM shoppinglist.store_categories sc
		JOIN shoppinglist.categories c ON c.id = sc.category_id
		WHERE sc.store_id = $1
		ORDER BY sc.sort_order`,
		storeID,
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

// SetStoreCategories fully replaces the store's category order. The array index
// of each category id becomes its sort_order. Only categories owned by userID
// are persisted; unknown or foreign ids are silently skipped.
func (r *ShoppingRepository) SetStoreCategories(
	ctx context.Context,
	userID string,
	storeID uuid.UUID,
	categoryIDs []uuid.UUID,
) error {
	if err := r.checkStoreOwnership(ctx, userID, storeID); err != nil {
		return err
	}

	_, err := r.db.Exec(ctx,
		`DELETE FROM shoppinglist.store_categories WHERE store_id = $1`,
		storeID,
	)
	if err != nil {
		return err
	}

	if len(categoryIDs) == 0 {
		return nil
	}

	//nolint:exhaustruct //other fields optional
	batch := &pgx.Batch{}
	for i, categoryID := range categoryIDs {
		batch.Queue(`
			INSERT INTO shoppinglist.store_categories (store_id, category_id, sort_order)
			SELECT $1, c.id, $3
			FROM shoppinglist.categories c
			WHERE c.id = $2 AND c.user_id = $4`,
			storeID, categoryID, i, userID,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()
	for range categoryIDs {
		if _, err = br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *ShoppingRepository) checkStoreOwnership(
	ctx context.Context,
	userID string,
	storeID uuid.UUID,
) error {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM shoppinglist.stores
			WHERE id = $1 AND user_id = $2
		)`,
		storeID, userID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Store not found",
		}
	}
	return nil
}
