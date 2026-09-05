package trains_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

// TestSearchStations_Handler_Success drives the handler over real HTTP
// against stationsFeed (stations_test.go), verifying the protoStation field
// mapping.
func TestSearchStations_Handler_Success(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))

	client := newTrainsTestClient(t)
	req := connect.NewRequest(&trainsv1.SearchStationsRequest{Query: "rav"})

	resp, err := client.SearchStations(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetStations(), 1)
	assert.Equal(t, "SB", resp.Msg.GetStations()[0].GetStopId())
	assert.Equal(t, "Bravo", resp.Msg.GetStations()[0].GetName())
}

// TestGetFeedInfo_Handler_Success drives GetFeedInfo over real HTTP after a
// static import, verifying the feed_version field mapping used to drive the
// required CC BY attribution string on /trains.
func TestGetFeedInfo_Handler_Success(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Services.StaticImport.Import(ctx))

	client := newTrainsTestClient(t)
	req := connect.NewRequest(&trainsv1.GetFeedInfoRequest{})
	resp, err := client.GetFeedInfo(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.GetFeedVersion())
}
