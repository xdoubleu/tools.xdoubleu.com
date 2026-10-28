package movies_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func TestErrorCodes(t *testing.T) {
	client := newClient(t, userID, nil)
	ctx := context.Background()
	unknown := uuid.NewString()
	rating := int32(9)

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"get unknown title", func() error {
			_, err := client.GetTitle(ctx, &moviesv1.GetTitleRequest{Id: unknown})
			return err
		}, connect.CodeNotFound},
		{"invalid title id", func() error {
			_, err := client.GetTitle(ctx, &moviesv1.GetTitleRequest{Id: "nope"})
			return err
		}, connect.CodeInvalidArgument},
		{"invalid media type", func() error {
			_, err := client.AddTitle(ctx, &moviesv1.AddTitleRequest{
				MediaType: "book", TmdbId: 1, Status: "want",
			})
			return err
		}, connect.CodeInvalidArgument},
		{"rating out of range", func() error {
			_, err := client.SetRating(ctx, &moviesv1.SetRatingRequest{
				Id: unknown, Rating: &rating,
			})
			return err
		}, connect.CodeInvalidArgument},
		{"tmdb not configured", func() error {
			_, err := client.SearchTitles(
				ctx, &moviesv1.SearchTitlesRequest{Query: "dune"},
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
