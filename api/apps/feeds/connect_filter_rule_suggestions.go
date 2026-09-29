package feeds

import (
	"context"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func protoFilterRuleSuggestion(
	s models.FilterRuleSuggestion,
) *feedsv1.FilterRuleSuggestion {
	return &feedsv1.FilterRuleSuggestion{
		FeedId:    s.FeedID.String(),
		FeedTitle: s.FeedTitle,
		FeedUrl:   s.FeedURL,
		Category:  s.Category,
		ItemCount: int32(s.ItemCount), //nolint:gosec // item counts fit int32
		ReadCount: int32(s.ReadCount), //nolint:gosec // item counts fit int32
		ReadRate:  float64(s.ReadCount) / float64(s.ItemCount),
	}
}

func (h *feedsConnectHandler) GetFilterRuleSuggestions(
	ctx context.Context,
	_ *connect.Request[feedsv1.GetFilterRuleSuggestionsRequest],
) (*connect.Response[feedsv1.GetFilterRuleSuggestionsResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	suggestions, err := h.app.Services.Feeds.GetFilterRuleSuggestions(ctx, user.ID)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}

	out := make([]*feedsv1.FilterRuleSuggestion, len(suggestions))
	for i, s := range suggestions {
		out[i] = protoFilterRuleSuggestion(s)
	}
	return connect.NewResponse(&feedsv1.GetFilterRuleSuggestionsResponse{
		Suggestions: out,
	}), nil
}

func (h *feedsConnectHandler) DismissFilterRuleSuggestion(
	ctx context.Context,
	req *connect.Request[feedsv1.DismissFilterRuleSuggestionRequest],
) (*connect.Response[feedsv1.DismissFilterRuleSuggestionResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.Msg.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	if err := h.app.Services.Feeds.DismissFilterRuleSuggestion(
		ctx, user.ID, feedID, req.Msg.Category,
	); err != nil {
		return nil, filterRuleErrorToConnect(err)
	}
	return connect.NewResponse(&feedsv1.DismissFilterRuleSuggestionResponse{}), nil
}
