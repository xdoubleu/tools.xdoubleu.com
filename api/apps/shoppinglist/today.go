package shoppinglist

import "context"

// CountCustomListItems returns how many items are on userID's shopping list.
func (app *ShoppingList) CountCustomListItems(
	ctx context.Context,
	userID string,
) (int, error) {
	items, err := app.services.Shopping.GetCustomList(ctx, userID)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}
