package taste_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/internal/consumption"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func TestShowsFeedPodcastsConsumption(t *testing.T) {
	const uid = "taste-shows-consumption-test-user"
	c := newShowsClient(t, uid, fakeITunes{})
	f := addShow(t, c, 1)

	resp, err := c.ListEvents(
		context.Background(), &tastev1.ListEventsRequest{Domain: "podcasts"},
	)
	require.NoError(t, err)
	require.Len(t, resp.Events, 1)
	e := resp.Events[0]
	assert.Equal(t, string(consumption.DomainPodcasts), e.Domain)
	assert.Equal(t, f.Id, e.ItemId)
	assert.Equal(t, "Hardcore History", e.Title)
	assert.Equal(t, "Dan Carlin", e.Creator)
	assert.Equal(t, int32(consumption.StrongPositive), e.Signal)
}

func TestShowsOutsideSharedScope(t *testing.T) {
	const uid = "taste-shows-shared-test-user"
	c := newShowsClient(t, uid, fakeITunes{})
	addShow(t, c, 1)

	resp, err := c.ListEvents(
		context.Background(),
		&tastev1.ListEventsRequest{Domain: "podcasts", Shared: true},
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Events)
}
