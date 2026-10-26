package shoppinglist_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

func TestErrorCodes(t *testing.T) {
	client := newShoppingClient(t)
	ctx := context.Background()
	unknown := uuid.NewString()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"delete unknown store", func() error {
			_, err := client.DeleteStore(
				ctx, &shoppinglistv1.DeleteStoreRequest{Id: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"categories of unknown store", func() error {
			_, err := client.GetStoreCategories(
				ctx, &shoppinglistv1.GetStoreCategoriesRequest{StoreId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"delete unknown category", func() error {
			_, err := client.DeleteCategory(
				ctx, &shoppinglistv1.DeleteCategoryRequest{Id: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"item into unknown category", func() error {
			_, err := client.SetItemCategory(
				ctx, &shoppinglistv1.SetItemCategoryRequest{
					Name: "Melk", CategoryId: unknown,
				},
			)
			return err
		}, connect.CodeNotFound},
		{"update unknown item", func() error {
			_, err := client.UpdateShoppingItem(
				ctx, &shoppinglistv1.UpdateShoppingItemRequest{
					ItemId: unknown, Name: "Melk", Amount: "1",
				},
			)
			return err
		}, connect.CodeNotFound},
		{"delete unknown item", func() error {
			_, err := client.DeleteShoppingItem(
				ctx, &shoppinglistv1.DeleteShoppingItemRequest{ItemId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"export unknown plan", func() error {
			_, err := client.GetMealPlanExportItems(
				ctx, &shoppinglistv1.GetMealPlanExportItemsRequest{PlanId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid store id", func() error {
			_, err := client.DeleteStore(
				ctx, &shoppinglistv1.DeleteStoreRequest{Id: "nope"},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"merge item into itself", func() error {
			_, err := client.SetItemAlias(
				ctx, &shoppinglistv1.SetItemAliasRequest{
					Name: "Ui", Canonical: " ui",
				},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"merge item into its own alias", func() error {
			_, err := client.SetItemAlias(
				ctx, &shoppinglistv1.SetItemAliasRequest{
					Name: "codes-a", Canonical: "codes-b",
				},
			)
			if err != nil {
				return err
			}
			_, err = client.SetItemAlias(
				ctx, &shoppinglistv1.SetItemAliasRequest{
					Name: "codes-b", Canonical: "codes-a",
				},
			)
			return err
		}, connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
