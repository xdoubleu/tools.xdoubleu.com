package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/internal/models"
)

func gameWithRate(name string, rate string) models.Game {
	//nolint:exhaustruct //only the fields the bucketing logic reads
	return models.Game{Name: name, CompletionRate: rate}
}

// TestBucketCompletionRates pins bucket boundaries and the
// zero/negative/unparsable exclusions.
func TestBucketCompletionRates(t *testing.T) {
	tests := []struct {
		name       string
		games      []models.Game
		wantBucket int // -1 means excluded from every bucket
	}{
		{
			name:       "zero rate excluded",
			games:      []models.Game{gameWithRate("a", "0.00")},
			wantBucket: -1,
		},
		{
			name:       "negative rate excluded",
			games:      []models.Game{gameWithRate("a", "-1.00")},
			wantBucket: -1,
		},
		{
			name:       "unparsable rate excluded",
			games:      []models.Game{gameWithRate("a", "n/a")},
			wantBucket: -1,
		},
		{
			name:       "just under first boundary",
			games:      []models.Game{gameWithRate("a", "9.99")},
			wantBucket: 0,
		},
		{
			name:       "first boundary",
			games:      []models.Game{gameWithRate("a", "10.00")},
			wantBucket: 1,
		},
		{
			name:       "just under 100",
			games:      []models.Game{gameWithRate("a", "99.99")},
			wantBucket: 9,
		},
		{
			name:       "exactly 100",
			games:      []models.Game{gameWithRate("a", "100.00")},
			wantBucket: 10,
		},
		{
			name:       "over 100 still last bucket",
			games:      []models.Game{gameWithRate("a", "150.00")},
			wantBucket: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts, bucketGames := bucketCompletionRates(tt.games)

			require.Len(t, counts, distributionBuckets)
			require.Len(t, bucketGames, distributionBuckets)

			totalCount := 0
			for i, c := range counts {
				totalCount += c
				if tt.wantBucket == i {
					assert.Equal(t, 1, c, "bucket %d", i)
					assert.Len(t, bucketGames[i], 1, "bucket %d", i)
				} else {
					assert.Equal(t, 0, c, "bucket %d", i)
					assert.Empty(t, bucketGames[i], "bucket %d", i)
				}
			}
			if tt.wantBucket == -1 {
				assert.Equal(t, 0, totalCount)
			} else {
				assert.Equal(t, 1, totalCount)
			}
		})
	}
}

// TestBucketCompletionRatesOrdersByRate: differing rates sort by rate, not
// name.
func TestBucketCompletionRatesOrdersByRate(t *testing.T) {
	games := []models.Game{
		gameWithRate("Apple", "58.00"),
		gameWithRate("Zebra", "50.00"),
	}

	_, bucketGames := bucketCompletionRates(games)

	require.Len(t, bucketGames[5], 2)
	assert.Equal(t, "Zebra", bucketGames[5][0].Name)
	assert.Equal(t, "Apple", bucketGames[5][1].Name)
}

// TestBucketCompletionRatesOrdersByNameOnTie: ties fall back to name.
func TestBucketCompletionRatesOrdersByNameOnTie(t *testing.T) {
	games := []models.Game{
		gameWithRate("Zeta", "52.00"),
		gameWithRate("Alpha", "52.00"),
		gameWithRate("Beta", "52.00"),
	}

	_, bucketGames := bucketCompletionRates(games)

	require.Len(t, bucketGames[5], 3)
	assert.Equal(t, "Alpha", bucketGames[5][0].Name)
	assert.Equal(t, "Beta", bucketGames[5][1].Name)
	assert.Equal(t, "Zeta", bucketGames[5][2].Name)
}
