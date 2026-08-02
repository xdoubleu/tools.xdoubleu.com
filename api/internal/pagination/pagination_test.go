package pagination_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/pagination"
)

func TestClamp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		limit         int32
		wantSafeLimit int
		wantSQLLimit  int
	}{
		{"zero defaults", 0, pagination.DefaultLimit, pagination.DefaultLimit + 1},
		{"negative defaults", -5, pagination.DefaultLimit, pagination.DefaultLimit + 1},
		{"within range", 10, 10, 11},
		{"above max caps", 500, pagination.MaxLimit, pagination.MaxLimit + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			safeLimit, sqlLimit := pagination.Clamp(tt.limit)
			assert.Equal(t, tt.wantSafeLimit, safeLimit)
			assert.Equal(t, tt.wantSQLLimit, sqlLimit)
		})
	}
}

func TestSplit(t *testing.T) {
	t.Parallel()

	t.Run("exact page has no more", func(t *testing.T) {
		t.Parallel()
		rows := []int{1, 2}
		page, hasMore := pagination.Split(rows, 2)
		assert.Equal(t, []int{1, 2}, page)
		assert.False(t, hasMore)
	})

	t.Run("extra row trimmed and flagged", func(t *testing.T) {
		t.Parallel()
		rows := []int{1, 2, 3}
		page, hasMore := pagination.Split(rows, 2)
		assert.Equal(t, []int{1, 2}, page)
		assert.True(t, hasMore)
	})
}
