package services

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/consumption"
)

// Digest limits keep the prompt small whatever the history's size.
const (
	digestLiked    = 25
	digestDisliked = 10
	digestRecent   = 20
	digestRanked   = 12
	recentWindow   = 90 * 24 * time.Hour
)

// Digest renders events as the compact text an LLM reads the user's taste
// from: per-domain likes and dislikes, recent items, and top tags and
// creators overall and recently, so drift shows.
func Digest(events []consumption.Event, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Today is %s.\n", now.Format(time.DateOnly))

	for _, d := range consumption.Domains() {
		domainEvents := Filter(events, d)
		if len(domainEvents) == 0 {
			continue
		}
		stats := domainStats(domainEvents)[slices.Index(consumption.Domains(), d)]
		fmt.Fprintf(&b, "\n## %s: %d consumed, %d liked, %d disliked\n",
			d, stats.Consumed, stats.Positive, stats.Negative)
		writeEvents(&b, "Liked most", strongest(domainEvents, true), digestLiked)
		writeEvents(&b, "Disliked", strongest(domainEvents, false), digestDisliked)
	}

	cutoff := now.Add(-recentWindow)
	recent := slices.DeleteFunc(slices.Clone(events), func(e consumption.Event) bool {
		return e.At.Before(cutoff)
	})
	SortNewestFirst(recent)
	writeEvents(&b, "\n## Last 90 days", recent, digestRecent)

	writeRanked(&b, "Top tags overall", rank(events, tags))
	writeRanked(&b, "Top tags, last 90 days", rank(recent, tags))
	writeRanked(&b, "Top creators overall", rank(events, creators))
	return b.String()
}

func tags(e consumption.Event) []string {
	return e.Tags
}

// strongest returns the liked (or else disliked) events, strongest and then
// newest first.
func strongest(events []consumption.Event, liked bool) []consumption.Event {
	out := slices.DeleteFunc(slices.Clone(events), func(e consumption.Event) bool {
		if liked {
			return e.Signal <= consumption.Neutral
		}
		return e.Signal >= consumption.Neutral
	})
	SortNewestFirst(out)
	slices.SortStableFunc(out, func(a, b consumption.Event) int {
		if liked {
			return cmp.Compare(b.Signal, a.Signal)
		}
		return cmp.Compare(a.Signal, b.Signal)
	})
	return out
}

func writeEvents(
	b *strings.Builder,
	heading string,
	events []consumption.Event,
	limit int,
) {
	if len(events) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", heading)
	for _, e := range events[:min(len(events), limit)] {
		line := e.Title
		if e.Creator != "" {
			line += " — " + e.Creator
		}
		if len(e.Tags) > 0 {
			line += " [" + strings.Join(e.Tags, ", ") + "]"
		}
		detail := fmt.Sprintf("%s, %+d %s", e.Domain, e.Signal, e.Reason)
		if !e.At.IsZero() {
			detail += ", " + e.At.Format("2006-01")
		}
		fmt.Fprintf(b, "- %s (%s)\n", line, detail)
	}
}

func writeRanked(b *strings.Builder, heading string, names []RankedName) {
	if len(names) == 0 {
		return
	}
	parts := make([]string, 0, digestRanked)
	for _, n := range names[:min(len(names), digestRanked)] {
		parts = append(parts, fmt.Sprintf("%s (%d)", n.Name, n.Positive))
	}
	fmt.Fprintf(b, "\n%s: %s\n", heading, strings.Join(parts, ", "))
}
