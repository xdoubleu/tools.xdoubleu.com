package trains_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/services"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

func TestSavedCommutes_Service_Validation(t *testing.T) {
	ctx := context.Background()
	testApp := newApp(t)
	_, err := testDB.Exec(ctx, "TRUNCATE trains.saved_commutes")
	require.NoError(t, err)

	created, err := testApp.Services.SavedCommutes.Create(
		ctx, userID, "c", brusselsNorth, liege,
	)
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
	_, err := testDB.Exec(ctx, "TRUNCATE trains.saved_commutes")
	require.NoError(t, err)

	client := newTrainsTestClient(t)

	var createdID string
	t.Run("create resolves station names and orders by position", func(t *testing.T) {
		res, cErr := client.CreateSavedCommute(ctx, &trainsv1.CreateSavedCommuteRequest{
			Label: "Home to work", OriginStopId: brusselsNorth, DestinationStopId: liege,
		})
		require.NoError(t, cErr)
		sc := res.GetSavedCommute()
		assert.Equal(t, "Home to work", sc.GetLabel())
		assert.Equal(t, brusselsNorth, sc.GetOrigin().GetStopId())
		assert.Equal(t, "Bruxelles-Nord", sc.GetOrigin().GetNameFr())
		assert.Equal(t, "Liège-Guillemins", sc.GetDestination().GetNameFr())
		assert.NotEmpty(t, sc.GetId())
		createdID = sc.GetId()

		second, cErr := client.CreateSavedCommute(ctx, &trainsv1.CreateSavedCommuteRequest{
			Label: "Work to home", OriginStopId: liege, DestinationStopId: brusselsNorth,
		})
		require.NoError(t, cErr)
		assert.Equal(t, int32(1), second.GetSavedCommute().GetPosition())
	})

	t.Run("list returns both, ordered", func(t *testing.T) {
		res, lErr := client.ListSavedCommutes(ctx, &trainsv1.ListSavedCommutesRequest{})
		require.NoError(t, lErr)
		list := res.GetSavedCommutes()
		require.Len(t, list, 2)
		assert.Equal(t, "Home to work", list[0].GetLabel())
		assert.Equal(t, "Work to home", list[1].GetLabel())
	})

	t.Run("duplicate pair conflicts", func(t *testing.T) {
		_, cErr := client.CreateSavedCommute(ctx, &trainsv1.CreateSavedCommuteRequest{
			Label: "dupe", OriginStopId: brusselsNorth, DestinationStopId: liege,
		})
		assert.Equal(t, connect.CodeAlreadyExists, connectCode(t, cErr))
	})

	t.Run("validation failures map to InvalidArgument", func(t *testing.T) {
		cases := []*trainsv1.CreateSavedCommuteRequest{
			{Label: "  ", OriginStopId: brusselsNorth, DestinationStopId: "008892007"},
			{Label: "same", OriginStopId: brusselsNorth, DestinationStopId: brusselsNorth},
			{Label: "ghost", OriginStopId: brusselsNorth, DestinationStopId: "nope"},
			{Label: "ghost", OriginStopId: "S8812005", DestinationStopId: liege},
		}
		for _, req := range cases {
			_, cErr := client.CreateSavedCommute(ctx, req)
			assert.Equal(t, connect.CodeInvalidArgument, connectCode(t, cErr))
		}
	})

	t.Run("update label and position", func(t *testing.T) {
		res, uErr := client.UpdateSavedCommute(ctx, &trainsv1.UpdateSavedCommuteRequest{
			Id: createdID, Label: "Renamed", Position: 5,
		})
		require.NoError(t, uErr)
		assert.Equal(t, "Renamed", res.GetSavedCommute().GetLabel())
		assert.Equal(t, int32(5), res.GetSavedCommute().GetPosition())
	})

	t.Run("update rejects bad id and missing row", func(t *testing.T) {
		_, uErr := client.UpdateSavedCommute(
			ctx,
			&trainsv1.UpdateSavedCommuteRequest{Id: "not-a-uuid", Label: "x"},
		)
		assert.Equal(t, connect.CodeInvalidArgument, connectCode(t, uErr))

		_, uErr = client.UpdateSavedCommute(ctx, &trainsv1.UpdateSavedCommuteRequest{
			Id: "00000000-0000-0000-0000-000000000000", Label: "x",
		})
		assert.Equal(t, connect.CodeNotFound, connectCode(t, uErr))
	})

	t.Run("delete removes the row", func(t *testing.T) {
		_, dErr := client.DeleteSavedCommute(
			ctx,
			&trainsv1.DeleteSavedCommuteRequest{Id: createdID},
		)
		require.NoError(t, dErr)

		_, dErr = client.DeleteSavedCommute(
			ctx,
			&trainsv1.DeleteSavedCommuteRequest{Id: createdID},
		)
		assert.Equal(t, connect.CodeNotFound, connectCode(t, dErr))

		res, lErr := client.ListSavedCommutes(ctx, &trainsv1.ListSavedCommutesRequest{})
		require.NoError(t, lErr)
		require.Len(t, res.GetSavedCommutes(), 1)
	})
}
