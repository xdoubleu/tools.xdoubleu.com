package dashboard_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
)

func TestErrorCodes(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)
	games := newPublicClient(t)
	reading := newPublicReadingClient(t)
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"unknown token", func() error {
			_, err := games.GetSharedSteam(
				ctx, &dashboardv1.GetSharedSteamRequest{Token: "unknown"},
			)
			return err
		}, connect.CodeNotFound},
		{"wrong-kind token", func() error {
			_, err := reading.GetSharedLibrary(
				ctx, &dashboardv1.GetSharedLibraryRequest{Token: publicGamesToken},
			)
			return err
		}, connect.CodeNotFound},
		{"unknown game", func() error {
			_, err := games.GetSharedSteamGame(
				ctx, &dashboardv1.GetSharedSteamGameRequest{
					Token: publicGamesToken, GameId: 999999,
				},
			)
			return err
		}, connect.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
