package taste_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func TestShowErrorCodes(t *testing.T) {
	client := newShowsClient(t, showsUserID, fakeITunes{})
	down := newShowsClient(t, showsUserID, downITunes{})
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"add unknown show", func() error {
			_, err := client.AddFavouriteShow(
				ctx, &tastev1.AddFavouriteShowRequest{ItunesId: 999999},
			)
			return err
		}, connect.CodeNotFound},
		{"remove unknown favourite", func() error {
			_, err := client.RemoveFavouriteShow(
				ctx, &tastev1.RemoveFavouriteShowRequest{Id: uuid.NewString()},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid itunes id", func() error {
			_, err := client.AddFavouriteShow(
				ctx, &tastev1.AddFavouriteShowRequest{ItunesId: 0},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"itunes down", func() error {
			_, err := down.SearchShows(
				ctx, &tastev1.SearchShowsRequest{Query: "history"},
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
