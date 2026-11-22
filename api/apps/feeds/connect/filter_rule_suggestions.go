package connect

import (
	"context"

	"tools.xdoubleu.com/apps/feeds/internal/protomap"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func (h *Handler) GetFilterRuleSuggestions(
	ctx context.Context,
	_ *feedsv1.GetFilterRuleSuggestionsRequest,
) (*feedsv1.GetFilterRuleSuggestionsResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	suggestions, err := h.feeds.GetFilterRuleSuggestions(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*feedsv1.FilterRuleSuggestion, len(suggestions))
	for i, s := range suggestions {
		out[i] = protomap.FilterRuleSuggestion(s)
	}
	return &feedsv1.GetFilterRuleSuggestionsResponse{
		Suggestions: out,
	}, nil
}

func (h *Handler) DismissFilterRuleSuggestion(
	ctx context.Context,
	req *feedsv1.DismissFilterRuleSuggestionRequest,
) (*feedsv1.DismissFilterRuleSuggestionResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	if err := h.feeds.DismissFilterRuleSuggestion(
		ctx, user.ID, feedID, req.Category,
	); err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.DismissFilterRuleSuggestionResponse{}, nil
}
