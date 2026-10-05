package services

// maxAlignEdits bounds the edit distance between a KEPUB document's text and
// the original's; kepubify changes a few units per document, not thousands.
const maxAlignEdits = 1000

// alignStarts maps ascending offsets in from onto to. An offset in text only
// from has maps to where that difference starts in to. Ambiguous characters
// match as late as possible, so a span right after text kepubify dropped
// lands on its own text rather than on the dropped text. ok is false when the
// texts differ by more than maxAlignEdits.
func alignStarts(from, to []uint16, starts []int) ([]int, bool) {
	// Only the common suffix is trimmed: it is matched late, as lateMatches
	// does; a trimmed prefix would match early.
	s := 0
	for s < len(from) && s < len(to) && from[len(from)-1-s] == to[len(to)-1-s] {
		s++
	}
	a, b := from[:len(from)-s], to[:len(to)-s]

	// at[i] is where a[i] lands in b.
	at := make([]int, len(a))
	if len(b) > 0 && len(a) > 0 {
		match, ok := lateMatches(a, b)
		if !ok {
			return nil, false
		}
		next := 0
		for i, j := range match {
			if j >= 0 {
				next = j + 1
				at[i] = j
				continue
			}
			at[i] = next
		}
	}

	out := make([]int, len(starts))
	for i, k := range starts {
		if k >= len(a) {
			out[i] = k - len(from) + len(to)
			continue
		}
		out[i] = at[k]
	}
	return out, true
}

// lateMatches is myersMatches run backwards, which matches as late as
// possible.
func lateMatches(a, b []uint16) ([]int, bool) {
	ra, rb := reversed(a), reversed(b)
	rm, ok := myersMatches(ra, rb, maxAlignEdits)
	if !ok {
		return nil, false
	}
	match := make([]int, len(a))
	for i, j := range rm {
		if j < 0 {
			match[len(a)-1-i] = -1
			continue
		}
		match[len(a)-1-i] = len(b) - 1 - j
	}
	return match, true
}

func reversed(s []uint16) []uint16 {
	r := make([]uint16, len(s))
	for i, c := range s {
		r[len(s)-1-i] = c
	}
	return r
}

// myersMatches returns, per index of a, the index of b it matches in a
// shortest edit script, or -1. ok is false past maxD edits.
func myersMatches(a, b []uint16, maxD int) ([]int, bool) {
	n, m := len(a), len(b)
	maxD = min(maxD, n+m)
	off := maxD + 1
	v := make([]int, 2*off+1)
	// trace[d] holds v[-d..d] after round d, for backtracking.
	var trace [][]int
	for d := 0; d <= maxD; d++ {
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
				return myersBacktrack(trace, n, m), true
			}
		}
		trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
	}
	return nil, false
}

func myersBacktrack(trace [][]int, n, m int) []int {
	match := make([]int, n)
	for i := range match {
		match[i] = -1
	}
	x, y := n, m
	for d := len(trace) - 1; d > 0; d-- {
		prev := trace[d-1]
		get := func(k int) int { return prev[k+d-1] }
		k := x - y
		prevK := k - 1
		if k == -d || (k != d && get(k-1) < get(k+1)) {
			prevK = k + 1
		}
		prevX := get(prevK)
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x--
			y--
			match[x] = y
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		x--
		y--
		match[x] = y
	}
	return match
}
