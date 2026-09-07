package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/services"
)

func TestEncodeDecodeJourneyID_RoundTrips(t *testing.T) {
	boardTime := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	alightTime := boardTime.Add(30 * time.Minute)
	legs := []services.LegRef{
		{
			TripShortName: "IC123",
			BoardStopID:   "A1",
			BoardTime:     boardTime,
			AlightStopID:  "B1",
			AlightTime:    alightTime,
		},
		{
			TripShortName: "IC456",
			BoardStopID:   "B1",
			BoardTime:     alightTime,
			AlightStopID:  "C1",
			AlightTime:    alightTime.Add(20 * time.Minute),
		},
	}

	id := services.EncodeJourneyID(legs)
	require.NotEmpty(t, id)

	decoded, err := services.DecodeJourneyID(id)
	require.NoError(t, err)
	require.Len(t, decoded, 2)
	assert.Equal(t, "IC123", decoded[0].TripShortName)
	assert.True(t, boardTime.Equal(decoded[0].BoardTime))
	assert.Equal(t, "IC456", decoded[1].TripShortName)
}

func TestDecodeJourneyID_RejectsGarbage(t *testing.T) {
	_, err := services.DecodeJourneyID("not-valid-base64!!!")
	require.ErrorIs(t, err, services.ErrInvalidJourneyID)
}

func TestDecodeJourneyID_RejectsValidBase64NonJourneyPayload(t *testing.T) {
	// Valid URL-safe base64, but not JSON shaped like []LegRef.
	_, err := services.DecodeJourneyID("aGVsbG8")
	require.ErrorIs(t, err, services.ErrInvalidJourneyID)
}

func TestDecodeJourneyID_RejectsEmptyLegList(t *testing.T) {
	id := services.EncodeJourneyID(nil)
	_, err := services.DecodeJourneyID(id)
	require.ErrorIs(t, err, services.ErrInvalidJourneyID)
}
