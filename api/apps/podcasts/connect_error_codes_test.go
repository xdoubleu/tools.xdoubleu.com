package podcasts_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	podcastsv1 "tools.xdoubleu.com/gen/podcasts/v1"
)

func TestErrorCodes(t *testing.T) {
	client := newClient(t, userID, fakeITunes{})
	down := newClient(t, userID, downITunes{})
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"add unknown show", func() error {
			_, err := client.AddFavourite(
				ctx, &podcastsv1.AddFavouriteRequest{ItunesId: 999999},
			)
			return err
		}, connect.CodeNotFound},
		{"remove unknown favourite", func() error {
			_, err := client.RemoveFavourite(
				ctx, &podcastsv1.RemoveFavouriteRequest{Id: uuid.NewString()},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid itunes id", func() error {
			_, err := client.AddFavourite(
				ctx, &podcastsv1.AddFavouriteRequest{ItunesId: 0},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"itunes down", func() error {
			_, err := down.SearchShows(
				ctx, &podcastsv1.SearchShowsRequest{Query: "history"},
			)
			return err
		}, connect.CodeUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
