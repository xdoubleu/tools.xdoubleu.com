package repositories

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPickupRefsReadsDependsOnSentences(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		tasks  []int64
		issues []int64
	}{
		{
			name:   "own line",
			body:   "Depends on: T-1\n\nDo it.",
			tasks:  []int64{1},
			issues: nil,
		},
		{
			name:   "own line without a colon, in bold",
			body:   "**Depends on** T-1 and T-2.\n",
			tasks:  []int64{1, 2},
			issues: nil,
		},
		{
			name:   "after a Part of sentence",
			body:   "Part of Hexagonal apps (T-748). Depends on: T-778 (convention and lint).",
			tasks:  []int64{778},
			issues: nil,
		},
		{
			name: "several ids, then a later sentence naming other tasks",
			body: "Part of T-722 (slice d). Depends on: T-763, T-764. " +
				"T-775 (factory jobs) waits on this task.",
			tasks:  []int64{763, 764},
			issues: nil,
		},
		{
			name: "a full stop inside parentheses does not end the sentence",
			body: "Depends on: T-800 (wardcase. ACP mode) and T-791 (adapters), " +
				"so this lands later. The page (T-802) consumes it.",
			tasks:  []int64{800, 791},
			issues: nil,
		},
		{
			name: "prose that mentions depends on mid-sentence",
			body: "Part of X (T-9). The slice that depends on T-5 comes later, " +
				"e.g. depends on T-6 too.",
			tasks:  nil,
			issues: nil,
		},
		{
			name:   "issue refs",
			body:   "Part of T-1. Depends on: #42 and T-3.",
			tasks:  []int64{3},
			issues: []int64{42},
		},
		{
			name:   "one per line",
			body:   "Part of X (T-9). Depends on: T-4.\n\nDepends on: T-5",
			tasks:  []int64{4, 5},
			issues: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, issues, _ := pickupRefs(tt.body)
			assert.Equal(t, tt.tasks, tasks)
			assert.Equal(t, tt.issues, issues)
		})
	}
}

func TestPickupRefsReadsTheIssueKey(t *testing.T) {
	_, _, key := pickupRefs("Imported. [github-issue:77]")
	assert.Equal(t, "77", key)
}
