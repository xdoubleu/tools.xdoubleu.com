package feeds_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func TestErrorCodes(t *testing.T) {
	client := newFeedsClient(t)
	ctx := context.Background()
	unknown := uuid.NewString()
	title := feedsv1.FilterRuleKind_FILTER_RULE_KIND_TITLE

	rule, ruleErr := client.CreateFilterRule(ctx, &feedsv1.CreateFilterRuleRequest{
		Kind: title, Value: "codes-" + unknown,
	})
	require.NoError(t, ruleErr)
	t.Cleanup(func() {
		_, _ = client.DeleteFilterRule(
			ctx, &feedsv1.DeleteFilterRuleRequest{RuleId: rule.Rule.Id},
		)
	})

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"update unknown feed", func() error {
			_, err := client.UpdateFeed(ctx, &feedsv1.UpdateFeedRequest{
				FeedId: unknown, Title: "x",
			})
			return err
		}, connect.CodeNotFound},
		{"refresh unknown feed", func() error {
			_, err := client.RefreshFeed(
				ctx, &feedsv1.RefreshFeedRequest{FeedId: unknown},
			)
			return err
		}, connect.CodeNotFound},
		{"invalid feed id", func() error {
			_, err := client.DeleteFeed(
				ctx, &feedsv1.DeleteFeedRequest{FeedId: "nope"},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"unsupported url", func() error {
			_, err := client.CreateFeed(ctx, &feedsv1.CreateFeedRequest{
				Url: "ftp://example.com/feed",
			})
			return err
		}, connect.CodeInvalidArgument},
		{"invalid filter rule", func() error {
			_, err := client.CreateFilterRule(
				ctx, &feedsv1.CreateFilterRuleRequest{Kind: title},
			)
			return err
		}, connect.CodeInvalidArgument},
		{"duplicate filter rule", func() error {
			_, err := client.CreateFilterRule(ctx, &feedsv1.CreateFilterRuleRequest{
				Kind: title, Value: "codes-" + unknown,
			})
			return err
		}, connect.CodeAlreadyExists},
		{"unhealthy feeds as non-admin", func() error {
			_, err := client.GetUnhealthyFeeds(
				ctx, &feedsv1.GetUnhealthyFeedsRequest{},
			)
			return err
		}, connect.CodePermissionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
