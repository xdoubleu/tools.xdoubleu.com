package games_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
)

func TestErrorCodes(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set("Cookie", accessToken.String())

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"unknown game", func() error {
			_, err := client.GetSteamGame(
				ctx, &gamesv1.GetSteamGameRequest{GameId: 999999999},
			)
			return err
		}, connect.CodeNotFound},
		{"favourite unknown game", func() error {
			_, err := client.SetGameFavourite(
				ctx, &gamesv1.SetGameFavouriteRequest{GameId: 999999999},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid bucket", func() error {
			_, err := client.GetSteamDistribution(
				ctx, &gamesv1.GetSteamDistributionRequest{Bucket: 99},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"non-numeric steam id", func() error {
			_, err := client.SaveIntegrations(ctx, &gamesv1.SaveIntegrationsRequest{
				Integrations: &gamesv1.Integrations{SteamUserId: "abc"},
			})
			return err
		}, connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
