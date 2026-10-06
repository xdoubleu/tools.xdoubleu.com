//nolint:testpackage // testing unexported alignment helpers
package services

import (
	"context"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func u16(s string) []uint16 { return utf16.Encode([]rune(s)) }

func TestAlignStarts(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		starts   []int
		want     []int
	}{
		{"equal", "abc def", "abc def", []int{0, 4, 7}, []int{0, 4, 7}},
		{"trailer only in from", "abc def\n", "abc def", []int{0, 4, 8}, []int{0, 4, 7}},
		{"leading newline only in to", "code. next", "\ncode. next",
			[]int{0, 6}, []int{1, 7}},
		{"text only in from", "a <p>x</p>z", "a xz", []int{0, 2, 10}, []int{0, 2, 3}},
		{"char removed from from", "bad  char. End.", "bad � char. End.",
			[]int{0, 11}, []int{0, 12}},
		{"both empty", "", "", []int{0}, []int{0}},
		{"from empty", "", "abc", []int{0}, []int{3}},
		{"span after dropped text", "Text more.", "Text raw  more.",
			[]int{0, 4}, []int{0, 9}},
		{"span after dropped space", "doc tail.", "doc  tail.", []int{3}, []int{4}},
		{"to empty", "abc", "", []int{0, 1, 3}, []int{0, 0, 0}},
		{"text only in from after a first-char match", "aZb", "ab",
			[]int{1}, []int{1}},
		{"match on to's last char", "Yx", "aY", []int{0}, []int{1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := alignStarts(context.Background(), u16(c.from), u16(c.to), c.starts)
			require.True(t, ok)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestAlignStarts_TooManyEdits(t *testing.T) {
	from := strings.Repeat("a", maxAlignEdits)
	to := strings.Repeat("b", maxAlignEdits)
	_, ok := alignStarts(context.Background(), u16(from), u16(to), []int{0})
	assert.False(t, ok)
}

func TestAlignStarts_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, ok := alignStarts(ctx, u16("abc"), u16("xbc"), []int{0})
	assert.False(t, ok)
}

func TestAlignStarts_ManySmallEditsInLongText(t *testing.T) {
	var from, to strings.Builder
	var starts, want []int
	for i := range 200 {
		starts = append(starts, len(u16(from.String())))
		want = append(want, len(u16(to.String())))
		from.WriteString("sentence. ")
		to.WriteString("sentence. ")
		if i%2 == 0 {
			to.WriteString("\n")
		}
	}
	got, ok := alignStarts(
		context.Background(), u16(from.String()), u16(to.String()), starts,
	)
	require.True(t, ok)
	assert.Equal(t, want, got)
}

func TestSpanMapCache_EvictsLeastRecentlyUsed(t *testing.T) {
	c := newSpanMapCache(2)
	k1 := spanMapKey{kepubID: uuid.New(), sourceID: uuid.New(), version: 1}
	k2 := spanMapKey{kepubID: uuid.New(), sourceID: uuid.New(), version: 1}
	k3 := spanMapKey{kepubID: uuid.New(), sourceID: uuid.New(), version: 1}
	m1, m2, m3 := &spanMap{docs: nil}, &spanMap{docs: nil}, &spanMap{docs: nil}

	c.add(k1, m1)
	c.add(k2, m2)
	got, ok := c.get(k1)
	require.True(t, ok)
	assert.Same(t, m1, got)

	c.add(k3, m3)
	_, ok = c.get(k2)
	assert.False(t, ok, "k2 was least recently used")
	_, ok = c.get(k1)
	assert.True(t, ok)

	c.add(k1, m2)
	got, _ = c.get(k1)
	assert.Same(t, m2, got, "re-adding replaces")

	other := k1
	other.version = 2
	_, ok = c.get(other)
	assert.False(t, ok, "converter version is part of the key")
}

func TestAlignStarts_OneSideEmptyIgnoresEditLimit(t *testing.T) {
	long := u16(strings.Repeat("x", maxAlignEdits+5))
	got, ok := alignStarts(context.Background(), long, nil, []int{0, 3})
	require.True(t, ok)
	assert.Equal(t, []int{0, 0}, got)

	got, ok = alignStarts(context.Background(), nil, long, []int{0})
	require.True(t, ok)
	assert.Equal(t, []int{len(long)}, got)
}

// lcsLen is the textbook LCS length, the reference for myersMatches.
func lcsLen(a, b []uint16) int {
	prev := make([]int, len(b)+1)
	for i := range a {
		cur := make([]int, len(b)+1)
		for j := range b {
			switch {
			case a[i] == b[j]:
				cur[j+1] = prev[j] + 1
			case prev[j+1] >= cur[j]:
				cur[j+1] = prev[j+1]
			default:
				cur[j+1] = cur[j]
			}
		}
		prev = cur
	}
	return prev[len(b)]
}

func randomText(r *rand.Rand, alphabet string) []uint16 {
	out := make([]uint16, r.IntN(10))
	for i := range out {
		out[i] = uint16(alphabet[r.IntN(len(alphabet))])
	}
	return out
}

// TestMyersMatches_OptimalAndValid: on random pairs the matching is a valid
// common subsequence of maximal length, and ok flips exactly at the edit
// distance.
func TestMyersMatches_OptimalAndValid(t *testing.T) {
	r := rand.New(rand.NewPCG(2158, 1))
	ctx := context.Background()
	for range 3000 {
		alphabet := []string{"ab", "abc", "abcd"}[r.IntN(3)]
		a, b := randomText(r, alphabet), randomText(r, alphabet)
		lcs := lcsLen(a, b)
		dist := len(a) + len(b) - 2*lcs

		match, ok := myersMatches(ctx, a, b, dist)
		require.True(t, ok, "%v %v", a, b)
		require.Len(t, match, len(a))
		matched, last := 0, -1
		for i, j := range match {
			if j < 0 {
				continue
			}
			require.Greater(t, j, last, "%v %v", a, b)
			require.Equal(t, a[i], b[j], "%v %v", a, b)
			last = j
			matched++
		}
		require.Equal(t, lcs, matched, "%v %v", a, b)

		if dist > 0 {
			_, ok = myersMatches(ctx, a, b, dist-1)
			require.False(t, ok, "%v %v", a, b)
		}

		late, ok := lateMatches(ctx, a, b)
		require.True(t, ok)
		matched, last = 0, -1
		for i, j := range late {
			if j >= 0 {
				require.Greater(t, j, last)
				require.Equal(t, a[i], b[j])
				last = j
				matched++
			}
		}
		require.Equal(t, lcs, matched, "%v %v", a, b)
	}
}
