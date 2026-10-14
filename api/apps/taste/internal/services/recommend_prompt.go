package services

import (
	"fmt"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

const (
	// picksPerList is how many picks the generator proposes per domain and
	// track; verification and ranking cut them to keepPerList.
	picksPerList = 5
	// maxFeedback caps the earlier picks' feedback in the generator's input.
	maxFeedback = 60
	// maxAlreadySuggested caps the earlier picks listed as already suggested.
	maxAlreadySuggested = 100
)

// feedbackLabels say what each feedback means to the generator.
//
//nolint:gochecknoglobals // read-only lookup table
var feedbackLabels = map[string]string{
	models.FeedbackHit:           "hit, loved it",
	models.FeedbackMiss:          "miss, didn't like it",
	models.FeedbackNotInterested: "not interested",
	models.FeedbackKnown:         "already knew it",
}

func generatorPrompt(domains []consumption.Domain) string {
	names := make([]string, len(domains))
	for i, d := range domains {
		names[i] = string(d)
	}
	return fmt.Sprintf(`You recommend what one person should read, play, watch `+
		`and listen to next.

First imagine four taste neighbours: people whose history and profile closely `+
		`match this person's but who differ in temperament.
- The purist goes deeper into exactly what the person loves.
- The explorer crosses into adjacent genres, forms and media.
- The contrarian champions overlooked, non-canonical and non-English work.
- The curator tracks what is new and acclaimed right now.
Let each neighbour propose picks, then merge them.

Search the web: for recent releases, lesser-known work, "if you liked X" `+
		`discussions and reviews, and to confirm each title exists as you name it. `+
		`Skip the famous canon this person has almost certainly met already. Never `+
		`propose anything in their history or in the "already suggested" list. `+
		`Learn from the feedback on earlier picks: lean into the directions that `+
		`were hits, especially on the broaden track, and away from the misses.

Two tracks:
- more: more of what they demonstrably like.
- broaden: deliberate stretches toward something their taste suggests they `+
		`would enjoy but have not explored. These may be motivated by a different `+
		`domain, e.g. a podcast because of the books they love.

Cover these domains: %s. Propose up to %d picks per domain and track. For `+
		`feeds, propose blogs, newsletters or sites with a feed and give the site `+
		`URL; leave url empty for every other domain. Give the title exactly as `+
		`published, the main creator (author, studio, director or host), the `+
		`release year or 0 when unsure, one sentence of why addressed to the `+
		`person that names specific items from their history, which neighbour `+
		`proposed it, and a score from 0 to 100: how likely they are to enjoy it `+
		`given their profile. On the broaden track also reward how well it `+
		`stretches them in a direction they are likely to welcome, and penalise `+
		`picks that are really "more of the same". Score honestly: a spread of `+
		`scores is more useful than all high ones.

Write the picks as a plain list, one per pick, with every field above. A `+
		`later step turns your list into structured data, so don't write JSON.`,
		strings.Join(names, ", "), picksPerList)
}

// formatterPrompt tells the format call to only restructure the research.
const formatterPrompt = `Turn the recommendations below into the requested ` +
	`JSON. Keep every pick exactly as written: do not add, drop, merge or ` +
	`reword any, and keep each why, neighbour and score as given. Use year 0 ` +
	`when none is given and an empty url when there is none.`

func generatorInput(
	events []consumption.Event,
	profile *models.Profile,
	previous, touched []models.Recommendation,
	now time.Time,
) string {
	var b strings.Builder
	if profile != nil && profile.Sections != nil {
		p := profile.Sections
		fmt.Fprintf(&b, "## Profile\n%s\n", p.Summary)
		writeList(&b, "Knows", p.Know)
		writeList(&b, "Likes", p.Like)
		writeList(&b, "Drifting toward", p.Drifting)
		writeList(&b, "Bounces off", p.Bounce)
		b.WriteString("\n")
	}
	b.WriteString("## History\n")
	b.WriteString(Digest(events, now))
	if len(previous) > 0 {
		b.WriteString("\n## Already suggested\n")
		for _, r := range previous {
			fmt.Fprintf(&b, "- %s (%s)\n", r.Title, r.Domain)
		}
	}
	n := 0
	for _, r := range touched {
		if r.Feedback == models.FeedbackNone || n == maxFeedback {
			continue
		}
		if n == 0 {
			b.WriteString("\n## Feedback on earlier picks\n")
		}
		n++
		fmt.Fprintf(&b, "- %s (%s, %s): %s\n",
			r.Title, r.Domain, r.Track, feedbackLabels[r.Feedback])
	}
	return b.String()
}

func writeList(b *strings.Builder, heading string, items []string) {
	if len(items) > 0 {
		fmt.Fprintf(b, "%s: %s\n", heading, strings.Join(items, "; "))
	}
}

func candidatesSchema() map[string]any {
	str := typeSchema("string")
	pick := objectSchema(map[string]any{
		"domain": enumSchema(
			"books", "feeds", "games", "movies", "series", "podcasts",
		),
		"track":     enumSchema(models.TrackMore, models.TrackBroaden),
		"title":     str,
		"creator":   str,
		"year":      typeSchema("integer"),
		"url":       str,
		"why":       str,
		"neighbour": str,
		"score":     typeSchema("integer"),
	})
	return objectSchema(map[string]any{"picks": arraySchema(pick)})
}
