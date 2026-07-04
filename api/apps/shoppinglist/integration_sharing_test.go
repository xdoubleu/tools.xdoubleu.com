package shoppinglist_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
)

// grantListAccess stages a sharing grant directly in the DB: owner shares their
// list with the test user (the mock auth always authenticates as userID, so the
// recipient side must be set up in the database).
func grantListAccess(t *testing.T, owner string, canEdit bool) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO shoppinglist.shoppinglist_access (owner_user_id, user_id, can_edit)
		VALUES ($1, $2, $3)
		ON CONFLICT (owner_user_id, user_id) DO UPDATE SET can_edit = EXCLUDED.can_edit`,
		owner, userID, canEdit,
	)
	require.NoError(t, err)
}

func TestShareShoppingList_RequiresContact(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.ShareShoppingList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ShareShoppingListRequest{ContactUserId: ""}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestShareShoppingList_RejectsSelf(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.ShareShoppingList(
		t.Context(),
		connect.NewRequest(
			&shoppinglistv1.ShareShoppingListRequest{ContactUserId: userID},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUnshareShoppingList_RequiresTarget(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.UnshareShoppingList(
		t.Context(),
		connect.NewRequest(
			&shoppinglistv1.UnshareShoppingListRequest{TargetUserId: ""},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestShareShoppingList_ListAndUnshare(t *testing.T) {
	client := newShoppingClient(t)

	_, err := client.ShareShoppingList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ShareShoppingListRequest{
			ContactUserId: "sl-target", CanEdit: true,
		}),
	)
	require.NoError(t, err)

	shares, err := client.ListShoppingListShares(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListShoppingListSharesRequest{}),
	)
	require.NoError(t, err)
	var found bool
	for _, s := range shares.Msg.Shares {
		if s.UserId == "sl-target" {
			found = true
			assert.True(t, s.CanEdit)
		}
	}
	require.True(t, found)

	_, err = client.UnshareShoppingList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.UnshareShoppingListRequest{
			TargetUserId: "sl-target",
		}),
	)
	require.NoError(t, err)

	after, err := client.ListShoppingListShares(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListShoppingListSharesRequest{}),
	)
	require.NoError(t, err)
	for _, s := range after.Msg.Shares {
		assert.NotEqual(t, "sl-target", s.UserId)
	}
}

func TestListAccessibleLists_IncludesSelfAndShared(t *testing.T) {
	grantListAccess(t, "sl-owner-2", true)

	client := newShoppingClient(t)
	resp, err := client.ListAccessibleLists(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListAccessibleListsRequest{}),
	)
	require.NoError(t, err)

	var hasSelf, hasShared bool
	for _, o := range resp.Msg.Owners {
		if o.IsSelf && o.UserId == userID {
			hasSelf = true
		}
		if o.UserId == "sl-owner-2" {
			hasShared = true
			assert.True(t, o.CanEdit)
		}
	}
	assert.True(t, hasSelf, "self list must be present")
	assert.True(t, hasShared, "shared list must be present")
}

func TestResolveOwner_NoAccessDenied(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.GetCustomList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetCustomListRequest{
			OwnerUserId: "stranger-owner",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestResolveOwner_SharedViewCanReadButNotWrite(t *testing.T) {
	const owner = "sl-owner-view"
	grantListAccess(t, owner, false)
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO shoppinglist.custom_items (user_id, name, amount, unit)
		VALUES ($1, 'Shared Apples', 3, 'pc')`,
		owner,
	)
	require.NoError(t, err)

	client := newShoppingClient(t)

	// Read works.
	resp, err := client.GetCustomList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetCustomListRequest{OwnerUserId: owner}),
	)
	require.NoError(t, err)
	var names []string
	for _, item := range resp.Msg.Items {
		names = append(names, item.Name)
	}
	assert.Contains(t, names, "Shared Apples")

	// Write is denied for a view-only grant.
	_, err = client.CreateShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.CreateShoppingItemRequest{
			Name: "Nope", Amount: "1", Unit: "", OwnerUserId: owner,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestResolveOwner_SharedEditCanWrite(t *testing.T) {
	const owner = "sl-owner-edit"
	grantListAccess(t, owner, true)

	client := newShoppingClient(t)
	resp, err := client.CreateShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.CreateShoppingItemRequest{
			Name: "Edit Allowed", Amount: "1", Unit: "pc", OwnerUserId: owner,
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Edit Allowed", resp.Msg.Item.Name)

	// The item is written to the owner's list, not the caller's own list.
	ownerList, err := client.GetCustomList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetCustomListRequest{OwnerUserId: owner}),
	)
	require.NoError(t, err)
	var names []string
	for _, item := range ownerList.Msg.Items {
		names = append(names, item.Name)
	}
	assert.Contains(t, names, "Edit Allowed")
}
