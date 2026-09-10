package trains_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/services"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

func connectCode(t *testing.T, err error) connect.Code {
	t.Helper()
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	return connErr.Code()
}

func TestSavedCommutes_Service_Validation(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	_, err := testDB.Exec(ctx, "TRUNCATE trains.saved_commutes")
	require.NoError(t, err)

	created, err := testApp.Services.SavedCommutes.Create(ctx, userID, "c", "SA", "SB")
	require.NoError(t, err)

	_, err = testApp.Services.SavedCommutes.Update(ctx, userID, created.ID, "ok", -1)
	require.ErrorIs(t, err, services.ErrInvalidCommute)

	_, err = testApp.Services.SavedCommutes.Update(ctx, userID, created.ID, "  ", 0)
	require.ErrorIs(t, err, services.ErrInvalidCommute)

	require.NoError(t,
		testApp.Services.SavedCommutes.Delete(ctx, userID, created.ID))
}

func TestSavedCommutes_Handler(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))
	_, err := testDB.Exec(ctx, "TRUNCATE trains.saved_commutes")
	require.NoError(t, err)

	client := newTrainsTestClient(t)

	var createdID string
	t.Run("create resolves station names and orders by position", func(t *testing.T) {
		res, cErr := client.CreateSavedCommute(ctx, connect.NewRequest(
			&trainsv1.CreateSavedCommuteRequest{
				Label: "Home to work", OriginStopId: "SA", DestinationStopId: "SB",
			},
		))
		require.NoError(t, cErr)
		sc := res.Msg.GetSavedCommute()
		assert.Equal(t, "Home to work", sc.GetLabel())
		assert.Equal(t, "SA", sc.GetOrigin().GetStopId())
		assert.Equal(t, "Alpha", sc.GetOrigin().GetNameFr())
		assert.Equal(t, "Bravo", sc.GetDestination().GetNameFr())
		assert.NotEmpty(t, sc.GetId())
		createdID = sc.GetId()

		second, cErr := client.CreateSavedCommute(ctx, connect.NewRequest(
			&trainsv1.CreateSavedCommuteRequest{
				Label: "Work to home", OriginStopId: "SB", DestinationStopId: "SA",
			},
		))
		require.NoError(t, cErr)
		assert.Equal(t, int32(1), second.Msg.GetSavedCommute().GetPosition())
	})

	t.Run("list returns both, ordered", func(t *testing.T) {
		res, lErr := client.ListSavedCommutes(ctx, connect.NewRequest(
			&trainsv1.ListSavedCommutesRequest{},
		))
		require.NoError(t, lErr)
		list := res.Msg.GetSavedCommutes()
		require.Len(t, list, 2)
		assert.Equal(t, "Home to work", list[0].GetLabel())
		assert.Equal(t, "Work to home", list[1].GetLabel())
	})

	t.Run("duplicate pair conflicts", func(t *testing.T) {
		_, cErr := client.CreateSavedCommute(ctx, connect.NewRequest(
			&trainsv1.CreateSavedCommuteRequest{
				Label: "dupe", OriginStopId: "SA", DestinationStopId: "SB",
			},
		))
		assert.Equal(t, connect.CodeAlreadyExists, connectCode(t, cErr))
	})

	t.Run("validation failures map to InvalidArgument", func(t *testing.T) {
		cases := []*trainsv1.CreateSavedCommuteRequest{
			{Label: "  ", OriginStopId: "SA", DestinationStopId: "SC"},
			{Label: "same", OriginStopId: "SA", DestinationStopId: "SA"},
			{Label: "ghost", OriginStopId: "SA", DestinationStopId: "nope"},
			{Label: "ghost", OriginStopId: "A1", DestinationStopId: "SB"},
		}
		for _, req := range cases {
			_, cErr := client.CreateSavedCommute(ctx, connect.NewRequest(req))
			assert.Equal(t, connect.CodeInvalidArgument, connectCode(t, cErr))
		}
	})

	t.Run("update label and position", func(t *testing.T) {
		res, uErr := client.UpdateSavedCommute(ctx, connect.NewRequest(
			&trainsv1.UpdateSavedCommuteRequest{
				Id: createdID, Label: "Renamed", Position: 5,
			},
		))
		require.NoError(t, uErr)
		assert.Equal(t, "Renamed", res.Msg.GetSavedCommute().GetLabel())
		assert.Equal(t, int32(5), res.Msg.GetSavedCommute().GetPosition())
	})

	t.Run("update rejects bad id and missing row", func(t *testing.T) {
		_, uErr := client.UpdateSavedCommute(ctx, connect.NewRequest(
			&trainsv1.UpdateSavedCommuteRequest{Id: "not-a-uuid", Label: "x"},
		))
		assert.Equal(t, connect.CodeInvalidArgument, connectCode(t, uErr))

		_, uErr = client.UpdateSavedCommute(ctx, connect.NewRequest(
			&trainsv1.UpdateSavedCommuteRequest{
				Id: "00000000-0000-0000-0000-000000000000", Label: "x",
			},
		))
		assert.Equal(t, connect.CodeNotFound, connectCode(t, uErr))
	})

	t.Run("delete removes the row", func(t *testing.T) {
		_, dErr := client.DeleteSavedCommute(ctx, connect.NewRequest(
			&trainsv1.DeleteSavedCommuteRequest{Id: createdID},
		))
		require.NoError(t, dErr)

		_, dErr = client.DeleteSavedCommute(ctx, connect.NewRequest(
			&trainsv1.DeleteSavedCommuteRequest{Id: createdID},
		))
		assert.Equal(t, connect.CodeNotFound, connectCode(t, dErr))

		res, lErr := client.ListSavedCommutes(ctx, connect.NewRequest(
			&trainsv1.ListSavedCommutesRequest{},
		))
		require.NoError(t, lErr)
		require.Len(t, res.Msg.GetSavedCommutes(), 1)
	})
}
