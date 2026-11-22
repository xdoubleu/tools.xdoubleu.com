package connect

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func (h *Handler) GetStats(
	ctx context.Context,
	req *tastev1.GetStatsRequest,
) (*tastev1.GetStatsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	stats, rates, err := h.svc.Reads.Stats(ctx, sc)
	if err != nil {
		return nil, mapError(err)
	}
	out := protoStats(stats)
	for _, r := range rates {
		out.HitRates = append(out.HitRates, &tastev1.HitRate{
			Domain: r.Domain, Track: r.Track,
			Hits: count32(r.Hits), Misses: count32(r.Misses),
		})
	}
	return out, nil
}

func (h *Handler) ListEvents(
	ctx context.Context,
	req *tastev1.ListEventsRequest,
) (*tastev1.ListEventsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	page, hasMore, err := h.svc.Reads.Events(
		ctx, sc, consumption.Domain(req.Domain), req.Limit, req.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*tastev1.Event, 0, len(page))
	for _, e := range page {
		out = append(out, protoEvent(e))
	}
	return &tastev1.ListEventsResponse{Events: out, HasMore: hasMore}, nil
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
