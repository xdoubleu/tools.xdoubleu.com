package feeds

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/apps/feeds/internal/services"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/internal/database"
)

// filterRuleKinds maps proto kinds to models.FilterRuleKind*; UNSPECIFIED
// maps to "", which CreateFilterRule rejects.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var filterRuleKinds = map[feedsv1.FilterRuleKind]string{
	feedsv1.FilterRuleKind_FILTER_RULE_KIND_UNSPECIFIED: "",
	feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY:    models.FilterRuleKindCategory,
	feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE:       models.FilterRuleKindTitle,
}

func protoFilterRule(r models.FilterRule) *feedsv1.FilterRule {
	kind := feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE
	if r.Kind == models.FilterRuleKindCategory {
		kind = feedsv1.FilterRuleKind_FILTER_RULE_KIND_CATEGORY
	}
	feedID := ""
	if r.FeedID != nil {
		feedID = r.FeedID.String()
	}
	return &feedsv1.FilterRule{
		Id:            r.ID.String(),
		FeedId:        feedID,
		Kind:          kind,
		Value:         r.Value,
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		FilteredCount: int32(r.FilteredCount), //nolint:gosec // item counts fit int32
	}
}

func filterRuleErrorToConnect(err error) *connect.Error {
	switch {
	case errors.Is(err, services.ErrInvalidFilterRule):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, database.ErrResourceConflict):
		return connect.NewError(
			connect.CodeAlreadyExists,
			errors.New("filter rule already exists"),
		)
	default:
		return feedErrorToConnect(err)
	}
}

func (h *feedsConnectHandler) ListFilterRules(
	ctx context.Context,
	_ *connect.Request[feedsv1.ListFilterRulesRequest],
) (*connect.Response[feedsv1.ListFilterRulesResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	rules, err := h.app.Services.Feeds.ListFilterRules(ctx, user.ID)
	if err != nil {
		return nil, filterRuleErrorToConnect(err)
	}

	out := make([]*feedsv1.FilterRule, len(rules))
	for i, r := range rules {
		out[i] = protoFilterRule(r)
	}
	return connect.NewResponse(&feedsv1.ListFilterRulesResponse{Rules: out}), nil
}

func (h *feedsConnectHandler) CreateFilterRule(
	ctx context.Context,
	req *connect.Request[feedsv1.CreateFilterRuleRequest],
) (*connect.Response[feedsv1.CreateFilterRuleResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	var feedID *uuid.UUID
	if req.Msg.FeedId != "" {
		parsed, perr := parseFeedID(req.Msg.FeedId)
		if perr != nil {
			return nil, perr
		}
		feedID = &parsed
	}

	rule, err := h.app.Services.Feeds.CreateFilterRule(
		ctx, user.ID, feedID, filterRuleKinds[req.Msg.Kind], req.Msg.Value,
	)
	if err != nil {
		return nil, filterRuleErrorToConnect(err)
	}
	return connect.NewResponse(&feedsv1.CreateFilterRuleResponse{
		Rule: protoFilterRule(*rule),
	}), nil
}

func (h *feedsConnectHandler) DeleteFilterRule(
	ctx context.Context,
	req *connect.Request[feedsv1.DeleteFilterRuleRequest],
) (*connect.Response[feedsv1.DeleteFilterRuleResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	ruleID, err := uuid.Parse(req.Msg.RuleId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid rule ID"),
		)
	}

	if err = h.app.Services.Feeds.DeleteFilterRule(ctx, user.ID, ruleID); err != nil {
		return nil, filterRuleErrorToConnect(err)
	}
	return connect.NewResponse(&feedsv1.DeleteFilterRuleResponse{}), nil
}
