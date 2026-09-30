package services

import (
	"math"
	"strings"
)

const (
	// minUnitChars is the shortest stream run trusted as a line unit; a
	// shorter run (a superscript, a stray symbol) has no reliable baseline.
	minUnitChars = 3
	// unitBreakGapRatio ends a unit at a forward gap wider than this many
	// character heights: text across a gap that wide is another block.
	unitBreakGapRatio = 3.0
	// baselineBucket rounds character bottoms before taking the modal one
	// as a unit's baseline.
	baselineBucket = 0.25
	// baselineTolRatio: two units share a line when their baselines are
	// within this fraction of the taller unit's character height.
	baselineTolRatio = 0.15
	// unitJoinGapRatio: units in different font families join a line only
	// across a word-sized gap (in page median character heights); a margin
	// note sharing a baseline with body text sits further off.
	unitJoinGapRatio = 1.5
)

// streamUnits splits chars, in PDFium stream order, into runs of
// consecutive characters on one PDFium line: a run ends at a line break, a
// stream discontinuity, a vertical jump (PDFium joins a hyphenated line to
// the next without a break), a backward jump, or a wide forward gap. Runs
// shorter than minUnitChars come back as loose characters.
func streamUnits(chars []pdfChar) ([][]pdfChar, []pdfChar) {
	var units [][]pdfChar
	var loose, cur []pdfChar
	flush := func() {
		if len(cur) >= minUnitChars {
			units = append(units, cur)
		} else {
			loose = append(loose, cur...)
		}
		cur = nil
	}
	var envTop, envBottom float64
	for _, c := range chars {
		if len(cur) > 0 && !continuesUnit(cur[len(cur)-1], c, envTop, envBottom) {
			flush()
		}
		if len(cur) == 0 {
			envTop, envBottom = c.top, c.bottom
		}
		envTop, envBottom = max(envTop, c.top), min(envBottom, c.bottom)
		cur = append(cur, c)
	}
	flush()
	return units, loose
}

// continuesUnit reports whether c extends the unit ending in prev, whose
// characters span [envBottom, envTop] vertically: an apostrophe above an
// x-height letter still overlaps the unit's ascenders.
func continuesUnit(prev, c pdfChar, envTop, envBottom float64) bool {
	if !streamAdjacent(prev, c) {
		return false
	}
	if c.bottom >= envTop || c.top <= envBottom {
		return false
	}
	h := max(prev.top-prev.bottom, c.top-c.bottom)
	return c.left >= prev.left-h && c.left-prev.right <= unitBreakGapRatio*h
}

// unitBaseline is the modal (bucketed) character bottom of a unit: letters
// without descenders all sit on the baseline.
func unitBaseline(unit []pdfChar) float64 {
	freq := map[float64]int{}
	best, bestCount := 0.0, 0
	for _, c := range unit {
		b := math.Round(c.bottom/baselineBucket) * baselineBucket
		freq[b]++
		if freq[b] > bestCount || (freq[b] == bestCount && b > best) {
			best, bestCount = b, freq[b]
		}
	}
	return best
}

// fontFamily is a font name up to its style suffix ("Minion" for
// "Minion-Italic").
func fontFamily(font string) string {
	family, _, _ := strings.Cut(font, "-")
	return family
}

// unitLine is a line under construction from baseline-matched units.
type unitLine struct {
	baseline, height float64
	units            [][]pdfChar
}

// mergeUnitsByBaseline joins units into lines: a unit joins a line whose
// baseline matches and which it continues (same font family, or only a
// word-sized gap away); otherwise it starts a line of its own.
func mergeUnitsByBaseline(units [][]pdfChar, medH float64) [][]pdfChar {
	var lines []*unitLine
	for _, u := range units {
		base, h := unitBaseline(u), medianCharHeight(u)
		var home *unitLine
		for _, l := range lines {
			tol := baselineTolRatio * max(h, l.height)
			if math.Abs(l.baseline-base) <= tol && joinsUnitLine(l, u, medH) {
				home = l
				break
			}
		}
		if home == nil {
			lines = append(lines, &unitLine{baseline: base, height: h, units: nil})
			home = lines[len(lines)-1]
		}
		home.units = append(home.units, u)
	}

	groups := make([][]pdfChar, len(lines))
	for i, l := range lines {
		for _, u := range l.units {
			groups[i] = append(groups[i], u...)
		}
	}
	return groups
}

func joinsUnitLine(l *unitLine, u []pdfChar, medH float64) bool {
	family := fontFamily(u[0].font)
	for _, other := range l.units {
		if fontFamily(other[0].font) == family {
			return true
		}
		gap := max(u[0].left-other[len(other)-1].right,
			other[0].left-u[len(u)-1].right)
		if gap <= unitJoinGapRatio*medH {
			return true
		}
	}
	return false
}

// attachToUnitLines adds each loose normal-height character to the unit
// line whose envelope (from its units alone, so attachments can't chain)
// overlaps it, closest y-midpoint first; the rest come back unattached.
func attachToUnitLines(
	groups [][]pdfChar, chars []pdfChar, medH float64,
) ([][]pdfChar, []pdfChar) {
	if len(groups) == 0 {
		return groups, chars
	}
	margin := clusterNormalCharsOverlapMarginRatio * medH
	envelopes := make([]pdfLine, len(groups))
	for i, g := range groups {
		envelopes[i] = buildLine(append([]pdfChar(nil), g...))
	}

	var rest []pdfChar
	for _, c := range chars {
		best, bestDist := -1, 0.0
		for i, env := range envelopes {
			if c.bottom > env.top+margin || c.top < env.bottom-margin {
				continue
			}
			if dist := math.Abs(env.yMid() - c.yMid()); best == -1 || dist < bestDist {
				best, bestDist = i, dist
			}
		}
		if best == -1 {
			rest = append(rest, c)
			continue
		}
		groups[best] = append(groups[best], c)
	}
	return groups, rest
}
