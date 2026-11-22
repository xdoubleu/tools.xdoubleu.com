package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/apps/feeds/internal/protomap"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
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

func (h *Handler) ListFilterRules(
	ctx context.Context,
	_ *feedsv1.ListFilterRulesRequest,
) (*feedsv1.ListFilterRulesResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	rules, err := h.feeds.ListFilterRules(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*feedsv1.FilterRule, len(rules))
	for i, r := range rules {
		out[i] = protomap.FilterRule(r)
	}
	return &feedsv1.ListFilterRulesResponse{Rules: out}, nil
}

func (h *Handler) CreateFilterRule(
	ctx context.Context,
	req *feedsv1.CreateFilterRuleRequest,
) (*feedsv1.CreateFilterRuleResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	var feedID *uuid.UUID
	if req.FeedId != "" {
		parsed, perr := parseFeedID(req.FeedId)
		if perr != nil {
			return nil, perr
		}
		feedID = &parsed
	}

	rule, err := h.feeds.CreateFilterRule(
		ctx, user.ID, feedID, filterRuleKinds[req.Kind], req.Value,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.CreateFilterRuleResponse{
		Rule: protomap.FilterRule(*rule),
	}, nil
}

func (h *Handler) DeleteFilterRule(
	ctx context.Context,
	req *feedsv1.DeleteFilterRuleRequest,
) (*feedsv1.DeleteFilterRuleResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	ruleID, err := uuid.Parse(req.RuleId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid rule ID",
		)
	}

	if err = h.feeds.DeleteFilterRule(ctx, user.ID, ruleID); err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.DeleteFilterRuleResponse{}, nil
}
