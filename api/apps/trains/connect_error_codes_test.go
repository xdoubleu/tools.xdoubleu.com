package trains_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

func TestErrorCodes(t *testing.T) {
	client := newTrainsTestClient(t)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"unknown station", func() error {
			_, err := client.SearchJourneys(ctx, &trainsv1.SearchJourneysRequest{
				OriginStopId: "000000000", DestinationStopId: "000000001",
			})
			return err
		}, connect.CodeNotFound},
		{"delete unknown commute", func() error {
			_, err := client.DeleteSavedCommute(
				ctx, &trainsv1.DeleteSavedCommuteRequest{Id: uuid.NewString()},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid time", func() error {
			_, err := client.SearchJourneys(
				ctx, &trainsv1.SearchJourneysRequest{Time: "noon"},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"invalid journey id", func() error {
			_, err := client.GetJourneyDetail(
				ctx, &trainsv1.GetJourneyDetailRequest{JourneyId: "nope"},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"invalid commute", func() error {
			_, err := client.CreateSavedCommute(
				ctx, &trainsv1.CreateSavedCommuteRequest{
					Label: "", OriginStopId: "1", DestinationStopId: "2",
				},
			)
			return err
		}, connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
