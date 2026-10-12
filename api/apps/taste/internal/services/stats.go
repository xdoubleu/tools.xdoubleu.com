package services

import (
	"cmp"
	"maps"
	"slices"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/consumption"
)

// topN caps the ranked tags and creators.
const topN = 15

// monthsShown is how many months the timeline covers, this one included.
const monthsShown = 12

// statsLocation is the timezone months are bucketed in; cmd/api embeds
// time/tzdata.
//
//nolint:gochecknoglobals // loaded once
var statsLocation = mustLoadLocation("Europe/Brussels")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

type DomainStats struct {
	Domain   consumption.Domain
	Consumed int
	Positive int
	Negative int
	// LastAt is the latest dated event; zero without one.
	LastAt time.Time
}

type MonthCounts struct {
	// Month is YYYY-MM.
	Month  string
	Counts map[consumption.Domain]int
}

// RankedName is a tag or creator with its positive item count and the
// domains those items came from, in display order.
type RankedName struct {
	Name     string
	Positive int
	Domains  []consumption.Domain
}

type Stats struct {
	Domains     []DomainStats
	Months      []MonthCounts
	TopTags     []RankedName
	TopCreators []RankedName
}

// BuildStats aggregates events as of now.
func BuildStats(events []consumption.Event, now time.Time) Stats {
	return Stats{
		Domains:     domainStats(events),
		Months:      monthCounts(events, now),
		TopTags:     rank(events, tags),
		TopCreators: rank(events, creators),
	}
}

func creators(e consumption.Event) []string {
	if e.Creator == "" {
		return nil
	}
	return []string{e.Creator}
}

func domainStats(events []consumption.Event) []DomainStats {
	byDomain := map[consumption.Domain]*DomainStats{}
	out := make([]DomainStats, 0, len(consumption.Domains()))
	for _, d := range consumption.Domains() {
		out = append(out, DomainStats{Domain: d}) //nolint:exhaustruct // counted below
	}
	for i := range out {
		byDomain[out[i].Domain] = &out[i]
	}
	for _, e := range events {
		ds, ok := byDomain[e.Domain]
		if !ok {
			continue
		}
		ds.Consumed++
		if e.Signal > 0 {
			ds.Positive++
		}
		if e.Signal < 0 {
			ds.Negative++
		}
		if e.At.After(ds.LastAt) {
			ds.LastAt = e.At
		}
	}
	return out
}

func monthCounts(events []consumption.Event, now time.Time) []MonthCounts {
	loc := statsLocation
	now = now.In(loc)
	first := time.Date(now.Year(), now.Month()-monthsShown+1, 1, 0, 0, 0, 0, loc)

	out := make([]MonthCounts, monthsShown)
	index := map[string]int{}
	for i := range out {
		key := first.AddDate(0, i, 0).Format("2006-01")
		out[i] = MonthCounts{Month: key, Counts: map[consumption.Domain]int{}}
		index[key] = i
	}
	for _, e := range events {
		if e.At.IsZero() {
			continue
		}
		if i, ok := index[e.At.In(loc).Format("2006-01")]; ok {
			out[i].Counts[e.Domain]++
		}
	}
	return out
}

// rank counts positive events per name, case-insensitively; each event counts
// once per name. The most common spelling is shown.
func rank(
	events []consumption.Event,
	names func(consumption.Event) []string,
) []RankedName {
	type tally struct {
		positive  int
		domains   map[consumption.Domain]bool
		spellings map[string]int
	}
	tallies := map[string]*tally{}
	for _, e := range events {
		if e.Signal <= 0 {
			continue
		}
		seen := map[string]bool{}
		for _, raw := range names(e) {
			name := strings.TrimSpace(raw)
			key := strings.ToLower(name)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			t, ok := tallies[key]
			if !ok {
				t = &tally{
					positive: 0, domains: map[consumption.Domain]bool{},
					spellings: map[string]int{},
				}
				tallies[key] = t
			}
			t.positive++
			t.domains[e.Domain] = true
			t.spellings[name]++
		}
	}

	out := make([]RankedName, 0, len(tallies))
	for _, t := range tallies {
		var domains []consumption.Domain
		for _, d := range consumption.Domains() {
			if t.domains[d] {
				domains = append(domains, d)
			}
		}
		out = append(out, RankedName{
			Name: mostCommon(t.spellings), Positive: t.positive, Domains: domains,
		})
	}
	slices.SortFunc(out, func(a, b RankedName) int {
		if c := cmp.Compare(b.Positive, a.Positive); c != 0 {
			return c
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out[:min(len(out), topN)]
}

// mostCommon returns the most used spelling, the first alphabetically on a
// tie.
func mostCommon(spellings map[string]int) string {
	sorted := slices.Sorted(maps.Keys(spellings))
	best := sorted[0]
	for _, s := range sorted[1:] {
		if spellings[s] > spellings[best] {
			best = s
		}
	}
	return best
}
