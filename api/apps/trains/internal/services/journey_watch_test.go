package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/services"
)

type recordingTopics struct{ ids []string }

func (r *recordingTopics) EnsureTopic(id string) { r.ids = append(r.ids, id) }

func TestJourneyWatchService_Watch(t *testing.T) {
	svc := services.New(mocks.NewFixtureClient(t), nil)
	topics := &recordingTopics{ids: nil}
	svc.WatchJourneys(topics)

	_, err := svc.JourneyWatch.Watch(context.Background(), "!!not-valid!!")
	require.ErrorIs(t, err, services.ErrInvalidJourneyID)
	assert.Empty(t, topics.ids, "a failing journey opens no topic")

	t0 := time.Now().UTC()
	id := services.EncodeJourneyID([]services.LegRef{{
		VehicleID: "BE.NMBS.IC530", BoardStopID: liege, BoardTime: t0,
		AlightStopID: brusselsNorth, AlightTime: t0.Add(time.Hour),
	}})
	detail, err := svc.JourneyWatch.Watch(context.Background(), id)
	require.NoError(t, err)
	assert.NotEmpty(t, detail.Legs)
	assert.Equal(t, []string{id}, topics.ids)
}
