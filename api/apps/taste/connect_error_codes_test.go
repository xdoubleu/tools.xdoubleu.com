package taste_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

func TestErrorCodes(t *testing.T) {
	unconfigured := newClient(t)
	_, configured := recsApp(t, sharedmocks.NewMockedAdminAuthService(userID))
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"save unknown pick", func() error {
			_, err := configured.SetSaved(ctx, &tastev1.SetSavedRequest{
				Id: uuid.NewString(), Saved: true,
			})
			return err
		}, connect.CodeNotFound},
		{"invalid pick id", func() error {
			_, err := configured.SetSaved(
				ctx, &tastev1.SetSavedRequest{Id: "nope", Saved: true},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"unknown domain", func() error {
			_, err := configured.ListEvents(
				ctx, &tastev1.ListEventsRequest{Domain: "music"},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"profile refresh without key", func() error {
			_, err := unconfigured.RefreshProfile(
				ctx, &tastev1.RefreshProfileRequest{},
			)
			return err
		}, connect.CodeFailedPrecondition},
		{"picks without a profile", func() error {
			_, err := configured.RefreshRecommendations(
				ctx, &tastev1.RefreshRecommendationsRequest{},
			)
			return err
		}, connect.CodeFailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
