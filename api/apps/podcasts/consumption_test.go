package podcasts_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/consumption"
)

func TestListConsumption(t *testing.T) {
	const uid = "podcasts-consumption-test-user"
	c := newClient(t, uid, fakeITunes{})
	f := add(t, c, 1)

	events, err := newApp(uid, fakeITunes{}).ListConsumption(context.Background(), uid)
	require.NoError(t, err)
	require.Len(t, events, 1)
	e := events[0]
	assert.Equal(t, consumption.DomainPodcasts, e.Domain)
	assert.Equal(t, f.Id, e.ItemID)
	assert.Equal(t, "Hardcore History", e.Title)
	assert.Equal(t, "Dan Carlin", e.Creator)
	assert.Equal(t, consumption.StrongPositive, e.Signal)
	assert.False(t, e.At.IsZero())
}

func TestListConsumption_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newApp(userID, fakeITunes{}).ListConsumption(ctx, userID)
	assert.Error(t, err)
}
