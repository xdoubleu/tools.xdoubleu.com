package taste

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
	"tools.xdoubleu.com/gen/taste/v1/tastev1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/consumption"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/pagination"
)

type tasteConnectHandler struct {
	app *Taste
}

var _ tastev1connect.TasteServiceHandler = (*tasteConnectHandler)(nil)

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(
			connect.CodeUnauthenticated, errors.New("user not authenticated"),
		)
	}
	return user.ID, nil
}

func (h *tasteConnectHandler) GetStats(
	ctx context.Context,
	_ *connect.Request[tastev1.GetStatsRequest],
) (*connect.Response[tastev1.GetStatsResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	events, err := h.app.log.Events(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	return connect.NewResponse(protoStats(services.BuildStats(events, time.Now()))), nil
}

func (h *tasteConnectHandler) ListEvents(
	ctx context.Context,
	req *connect.Request[tastev1.ListEventsRequest],
) (*connect.Response[tastev1.ListEventsResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	domain := consumption.Domain(req.Msg.Domain)
	if domain != "" && !services.IsDomain(domain) {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("unknown domain"),
		)
	}
	if req.Msg.Offset < 0 {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("offset must not be negative"),
		)
	}
	events, err := h.app.log.Events(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	events = services.Filter(events, domain)

	limit, _ := pagination.Clamp(req.Msg.Limit)
	start := min(int(req.Msg.Offset), len(events))
	end := min(start+limit, len(events))
	out := make([]*tastev1.Event, 0, end-start)
	for _, e := range events[start:end] {
		out = append(out, protoEvent(e))
	}
	return connect.NewResponse(&tastev1.ListEventsResponse{
		Events: out, HasMore: end < len(events),
	}), nil
}

func protoEvent(e consumption.Event) *tastev1.Event {
	return &tastev1.Event{
		Domain:  string(e.Domain),
		ItemId:  e.ItemID,
		Title:   e.Title,
		Creator: e.Creator,
		Tags:    e.Tags,
		At:      formatTime(e.At),
		Signal:  int32(e.Signal), //nolint:gosec // -2..2
		Reason:  e.Reason,
	}
}

func protoStats(s services.Stats) *tastev1.GetStatsResponse {
	domains := make([]*tastev1.DomainStats, len(s.Domains))
	for i, d := range s.Domains {
		domains[i] = &tastev1.DomainStats{
			Domain:   string(d.Domain),
			Consumed: count32(d.Consumed),
			Positive: count32(d.Positive),
			Negative: count32(d.Negative),
			LastAt:   formatTime(d.LastAt),
		}
	}
	months := make([]*tastev1.MonthCounts, len(s.Months))
	for i, m := range s.Months {
		var counts []*tastev1.DomainCount
		for _, d := range consumption.Domains() {
			if n := m.Counts[d]; n > 0 {
				counts = append(counts, &tastev1.DomainCount{
					Domain: string(d), Count: count32(n),
				})
			}
		}
		months[i] = &tastev1.MonthCounts{Month: m.Month, Counts: counts}
	}
	return &tastev1.GetStatsResponse{
		Domains:     domains,
		Months:      months,
		TopTags:     protoRanked(s.TopTags),
		TopCreators: protoRanked(s.TopCreators),
	}
}

func protoRanked(names []services.RankedName) []*tastev1.RankedName {
	out := make([]*tastev1.RankedName, len(names))
	for i, n := range names {
		domains := make([]string, len(n.Domains))
		for j, d := range n.Domains {
			domains[j] = string(d)
		}
		out[i] = &tastev1.RankedName{
			Name: n.Name, Positive: count32(n.Positive), Domains: domains,
		}
	}
	return out
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func count32(n int) int32 {
	return int32(n) //nolint:gosec // a user's item counts fit in int32
}
