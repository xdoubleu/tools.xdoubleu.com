// Package protomap maps the taste domain models to their proto messages, for
// the Connect and MCP adapters.
package protomap

import (
	"time"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/apps/taste/internal/services"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

// Event maps one consumption event.
func Event(e consumption.Event) *tastev1.Event {
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

// Events maps a page of consumption events.
func Events(page []consumption.Event, hasMore bool) *tastev1.ListEventsResponse {
	out := make([]*tastev1.Event, 0, len(page))
	for _, e := range page {
		out = append(out, Event(e))
	}
	return &tastev1.ListEventsResponse{Events: out, HasMore: hasMore}
}

// Stats maps the stats and the recommendation hit rates.
func Stats(s services.Stats, rates []models.HitRate) *tastev1.GetStatsResponse {
	out := stats(s)
	for _, r := range rates {
		out.HitRates = append(out.HitRates, &tastev1.HitRate{
			Domain: r.Domain, Track: r.Track,
			Hits: count32(r.Hits), Misses: count32(r.Misses),
		})
	}
	return out
}

func stats(s services.Stats) *tastev1.GetStatsResponse {
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
		TopTags:     ranked(s.TopTags),
		TopCreators: ranked(s.TopCreators),
	}
}

func ranked(names []services.RankedName) []*tastev1.RankedName {
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

// FillProfile copies a stored profile into the response.
func FillProfile(out *tastev1.GetProfileResponse, p *models.Profile) {
	if p.Sections != nil {
		out.Sections = &tastev1.ProfileSections{
			Summary:  p.Sections.Summary,
			Know:     p.Sections.Know,
			Like:     p.Sections.Like,
			Drifting: p.Sections.Drifting,
			Bounce:   p.Sections.Bounce,
		}
	}
	out.Model = p.Model
	out.GeneratedAt = TimePtr(p.GeneratedAt)
	out.Pending = p.Pending
	out.LastError = p.LastError
	out.NextRefreshAt = NextRefreshAt(p.RequestedAt)
}

// NextRefreshAt is when a refresh requested at requested is accepted again;
// empty when now.
func NextRefreshAt(requested *time.Time) string {
	if requested == nil {
		return ""
	}
	if next := requested.Add(services.RefreshCooldown); next.After(time.Now()) {
		return formatTime(next)
	}
	return ""
}

// TimePtr formats t as RFC 3339, or "" when nil.
func TimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

// Recommendation maps one pick.
func Recommendation(r models.Recommendation) *tastev1.Recommendation {
	sources := make([]*tastev1.Source, len(r.Sources))
	for i, s := range r.Sources {
		sources[i] = &tastev1.Source{Url: s.URL, Title: s.Title}
	}
	return &tastev1.Recommendation{
		Id:           r.ID,
		Domain:       r.Domain,
		Track:        r.Track,
		Title:        r.Title,
		Creator:      r.Creator,
		Year:         count32(r.Year),
		Url:          r.URL,
		ImageUrl:     r.ImageURL,
		Why:          r.Why,
		Sources:      sources,
		Score:        count32(r.Score),
		Feedback:     r.Feedback,
		Saved:        r.Saved,
		Accepted:     r.Accepted,
		OnMyServices: r.OnMyServices,
		CanAccept:    r.CanAccept(),
		GeneratedAt:  r.GeneratedAt.UTC().Format(time.RFC3339),
	}
}

// FavouriteShow maps one favourite show.
func FavouriteShow(s *models.Show) *tastev1.FavouriteShow {
	return &tastev1.FavouriteShow{
		Id:         s.ID.String(),
		ItunesId:   s.ITunesID,
		Title:      s.Title,
		Author:     s.Author,
		ArtworkUrl: s.ArtworkURL,
		AppleUrl:   s.AppleURL,
		AddedAt:    s.AddedAt.Format(time.RFC3339),
	}
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
