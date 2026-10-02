package shoppinglist_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

func TestCreateShoppingItem_ClientIDRepeatIsIdempotent(t *testing.T) {
	client := newShoppingClient(t)
	id := uuid.NewString()
	req := &shoppinglistv1.CreateShoppingItemRequest{
		Id: id, Name: "replayed", Amount: "2", Unit: "kg",
	}

	first, err := client.CreateShoppingItem(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(), "DELETE FROM shoppinglist.custom_items WHERE id = $1", id,
		)
	})
	second, err := client.CreateShoppingItem(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)

	assert.Equal(t, id, first.Msg.Item.Id)
	assert.Equal(t, first.Msg.Item, second.Msg.Item)

	list, err := client.GetCustomList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetCustomListRequest{}),
	)
	require.NoError(t, err)
	count := 0
	for _, item := range list.Msg.Items {
		if item.Id == id {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestCreateCategory_ClientIDRepeatIsIdempotent(t *testing.T) {
	client := newShoppingClient(t)
	id := uuid.NewString()
	req := &shoppinglistv1.CreateCategoryRequest{Id: id, Name: "Replayed category"}

	first, err := client.CreateCategory(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)
	second, err := client.CreateCategory(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)

	assert.Equal(t, id, first.Msg.Category.Id)
	assert.Equal(t, first.Msg.Category, second.Msg.Category)
}

func TestCreateStore_ClientIDRepeatIsIdempotent(t *testing.T) {
	client := newShoppingClient(t)
	id := uuid.NewString()
	req := &shoppinglistv1.CreateStoreRequest{Id: id, Name: "Replayed store"}

	first, err := client.CreateStore(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)
	second, err := client.CreateStore(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)

	assert.Equal(t, id, first.Msg.Store.Id)
	assert.Equal(t, first.Msg.Store, second.Msg.Store)
}

func TestCreateStore_ClientIDOwnedByAnotherUser(t *testing.T) {
	client := newShoppingClient(t)
	id := uuid.New()
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO shoppinglist.stores (id, user_id, name) VALUES ($1, $2, 'Theirs')`,
		id, uuid.NewString(),
	)
	require.NoError(t, err)

	_, err = client.CreateStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.CreateStoreRequest{
			Id: id.String(), Name: "Mine",
		}),
	)
	assertCode(t, err, connect.CodeNotFound)
}

func TestCreate_InvalidClientID(t *testing.T) {
	client := newShoppingClient(t)

	_, err := client.CreateShoppingItem(t.Context(), connect.NewRequest(
		&shoppinglistv1.CreateShoppingItemRequest{Id: "nope", Name: "x", Amount: "1"},
	))
	assertCode(t, err, connect.CodeInvalidArgument)

	_, err = client.CreateCategory(t.Context(), connect.NewRequest(
		&shoppinglistv1.CreateCategoryRequest{Id: "nope", Name: "x"},
	))
	assertCode(t, err, connect.CodeInvalidArgument)

	_, err = client.CreateStore(t.Context(), connect.NewRequest(
		&shoppinglistv1.CreateStoreRequest{Id: "nope", Name: "x"},
	))
	assertCode(t, err, connect.CodeInvalidArgument)
}
