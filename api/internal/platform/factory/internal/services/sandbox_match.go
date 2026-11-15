package services

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

var taskRefPattern = regexp.MustCompile(`\bT-(\d{1,18})\b`)

// similarTitle is the word overlap (Jaccard) at which two titles are one problem.
const similarTitle = 0.6

// findCovering returns the open task in the repo that already covers title: one
// it names by number, else a sandbox task with the same or a close title.
func findCovering(all []models.Task, repoID uuid.UUID, title string) *models.Task {
	open := func(t *models.Task) bool {
		return t.RepoID == repoID && t.Status != models.StatusDone
	}
	for _, m := range taskRefPattern.FindAllStringSubmatch(title, -1) {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			continue
		}
		for i := range all {
			if open(&all[i]) && all[i].Number == n {
				return &all[i]
			}
		}
	}
	words := titleWords(title)
	var best *models.Task
	bestScore := 0.0
	for i := range all {
		t := &all[i]
		if !open(t) || !IsSandboxTask(t) {
			continue
		}
		if strings.EqualFold(t.Title, title) {
			return t
		}
		if score := jaccard(words, titleWords(t.Title)); score >= similarTitle &&
			score > bestScore {
			best, bestScore = t, score
		}
	}
	return best
}

func titleWords(s string) map[string]bool {
	stopWords := map[string]bool{
		"sandbox": true, "the": true, "for": true, "and": true, "has": true,
		"have": true, "with": true, "not": true, "are": true, "was": true,
	}
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) > 2 && !stopWords[w] {
			out[w] = true
		}
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	shared := 0
	for w := range a {
		if b[w] {
			shared++
		}
	}
	if union := len(a) + len(b) - shared; union > 0 {
		return float64(shared) / float64(union)
	}
	return 0
}
