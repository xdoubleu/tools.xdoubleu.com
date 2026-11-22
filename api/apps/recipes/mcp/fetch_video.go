package mcp

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	recipesconnect "tools.xdoubleu.com/apps/recipes/connect"
)

const mcpFetchVideoDescription = "Title and description of a YouTube short " +
	"or video, to import the recipe in it. Extract the recipe and save it " +
	"with recipes_create_recipe, following its conventions, with the video " +
	"URL in the \"Bron:\" line. No transcript is available: if the " +
	"description has no usable recipe (it is only shown on screen or " +
	"spoken), ask the user for a screenshot. Instagram reels aren't " +
	"supported: ask the user to paste the caption or send a screenshot."

type mcpFetchVideoArgs struct {
	URL string `json:"url" jsonschema:"YouTube shorts, watch or youtu.be URL"`
}

func (h *tools) mcpFetchVideo(
	ctx context.Context, args mcpFetchVideoArgs,
) (proto.Message, error) {
	video, err := h.videos.Fetch(ctx, args.URL)
	if err != nil {
		return nil, recipesconnect.MapError(err)
	}

	return structpb.NewStruct(map[string]any{
		"platform":    video.Platform,
		"url":         video.URL,
		"title":       video.Title,
		"description": video.Description,
	})
}
