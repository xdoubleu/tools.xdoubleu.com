package trains_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

// TestSearchStations_Handler_Success checks protoStation mapping over HTTP.
func TestSearchStations_Handler_Success(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))

	client := newTrainsTestClient(t)
	req := connect.NewRequest(&trainsv1.SearchStationsRequest{Query: "rav"})

	resp, err := client.SearchStations(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetStations(), 1)
	assert.Equal(t, "SB", resp.Msg.GetStations()[0].GetStopId())
	assert.Equal(t, "Bravo-NL", resp.Msg.GetStations()[0].GetNameNl())
	assert.Equal(t, "Bravo", resp.Msg.GetStations()[0].GetNameFr())
	assert.Equal(t, "Bravo-EN", resp.Msg.GetStations()[0].GetNameEn())
}

// TestGetFeedInfo_Handler_Success checks feed_version and translation
// coverage mapping. Uses ImportFeed directly: the shared mock serves a real
// body only once per test binary.
func TestGetFeedInfo_Handler_Success(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testApp.Repositories.Feed.ImportFeed(ctx, stationsFeed()))

	client := newTrainsTestClient(t)
	req := connect.NewRequest(&trainsv1.GetFeedInfoRequest{})
	resp, err := client.GetFeedInfo(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "2026-08-31", resp.Msg.GetFeedVersion())
	importedAt, err := time.Parse(time.RFC3339, resp.Msg.GetImportedAt())
	require.NoError(t, err, "imported_at is an RFC3339 timestamp")
	assert.WithinDuration(t, time.Now(), importedAt, time.Minute)

	translations := resp.Msg.GetTranslations()
	require.NotNil(t, translations)
	assert.Equal(t, int32(2), translations.GetTranslatedStopsNl())
	assert.Equal(t, int32(0), translations.GetTranslatedStopsFr())
	assert.Equal(t, int32(2), translations.GetTranslatedStopsEn())
	assert.Equal(t, int32(5), translations.GetRows())
	assert.Equal(t, int32(1), translations.GetRowsUnmatched())
}
