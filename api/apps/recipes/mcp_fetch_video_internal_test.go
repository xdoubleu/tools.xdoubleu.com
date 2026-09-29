package recipes

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"tools.xdoubleu.com/apps/recipes/pkg/videofetch"
)

type stubVideoFetcher struct {
	video *videofetch.Video
	err   error
	got   string
}

func (s *stubVideoFetcher) Fetch(
	_ context.Context, rawURL string,
) (*videofetch.Video, error) {
	s.got = rawURL
	return s.video, s.err
}

func handlerWithFetcher(f videoFetcher) *recipesConnectHandler {
	//nolint:exhaustruct // only the fetcher is used
	return &recipesConnectHandler{app: &Recipes{videos: f}}
}

func TestMCPFetchVideo(t *testing.T) {
	stub := &stubVideoFetcher{
		video: &videofetch.Video{
			Platform:    "youtube",
			URL:         "https://www.youtube.com/shorts/XsipAaImDVc",
			Title:       "White Sauce Pasta",
			Description: "INGREDIENTS:\n2 cups Penne Pasta",
		},
		err: nil,
		got: "",
	}

	msg, err := handlerWithFetcher(stub).mcpFetchVideo(
		context.Background(),
		mcpFetchVideoArgs{URL: "https://youtu.be/XsipAaImDVc"},
	)
	require.NoError(t, err)
	assert.Equal(t, "https://youtu.be/XsipAaImDVc", stub.got)

	result, ok := msg.(*structpb.Struct)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"platform":    "youtube",
		"url":         "https://www.youtube.com/shorts/XsipAaImDVc",
		"title":       "White Sauce Pasta",
		"description": "INGREDIENTS:\n2 cups Penne Pasta",
	}, result.AsMap())
}

func TestMCPFetchVideo_Errors(t *testing.T) {
	tests := []struct {
		err  error
		code connect.Code
	}{
		{videofetch.ErrInstagram, connect.CodeInvalidArgument},
		{videofetch.ErrUnsupportedURL, connect.CodeInvalidArgument},
		{videofetch.ErrNotFound, connect.CodeNotFound},
		{fmt.Errorf("%w: HTTP 429", videofetch.ErrFetch), connect.CodeUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			_, err := handlerWithFetcher(
				&stubVideoFetcher{video: nil, err: tt.err, got: ""},
			).mcpFetchVideo(context.Background(), mcpFetchVideoArgs{URL: "x"})
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.code, connect.CodeOf(err))
		})
	}
}
