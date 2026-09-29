package recipes

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"tools.xdoubleu.com/apps/recipes/pkg/videofetch"
)

const mcpFetchVideoDescription = "Title and description of a YouTube short " +
	"or video, to import the recipe in it. Extract the recipe and save it " +
	"with recipes_create_recipe, following its conventions, with the video " +
	"URL in the \"Bron:\" line. No transcript is available: if the " +
	"description has no usable recipe (it is only shown on screen or " +
	"spoken), ask the user for a screenshot. Instagram reels aren't " +
	"supported: ask the user to paste the caption or send a screenshot."

type videoFetcher interface {
	Fetch(ctx context.Context, rawURL string) (*videofetch.Video, error)
}

type mcpFetchVideoArgs struct {
	URL string `json:"url" jsonschema:"YouTube shorts, watch or youtu.be URL"`
}

func (h *recipesConnectHandler) mcpFetchVideo(
	ctx context.Context, args mcpFetchVideoArgs,
) (proto.Message, error) {
	video, err := h.app.videos.Fetch(ctx, args.URL)
	switch {
	case errors.Is(err, videofetch.ErrInstagram),
		errors.Is(err, videofetch.ErrUnsupportedURL):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, videofetch.ErrNotFound):
		return nil, connect.NewError(connect.CodeNotFound, err)
	case err != nil:
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	return structpb.NewStruct(map[string]any{
		"platform":    video.Platform,
		"url":         video.URL,
		"title":       video.Title,
		"description": video.Description,
	})
}
