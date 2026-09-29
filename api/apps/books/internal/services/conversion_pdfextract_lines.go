package services

import (
	"math"
	"sort"
	"strings"
)

// normalCharHeightRatio distinguishes an ordinary letter — whose box spans
// most of the page's median character height — from small punctuation
// (commas, apostrophes, quotation marks, ...) whose box is much shorter and
// sits off to one side of the baseline: low for commas/descenders, high for
// apostrophes/quotes. Only normal-height characters take part in line
// clustering (clusterNormalChars); small characters are attached afterwards
// to whichever established line they're vertically closest to
// (attachSmallChars). Letting punctuation's own off-center box influence
// clustering is what caused #594 (commas split into their own line) and
// #618 (apostrophes/quotes did the same, the opposite direction) — and
// widening the clustering window to tolerate both directions at once (tried
// while fixing #618) let unrelated lines merge into one, scrambling reading
// order within the merged group.
const normalCharHeightRatio = 0.7

// groupLines clusters chars into lines (step 1 of the text algorithm).
// Runs of characters PDFium streamed together on one baseline (streamUnits)
// join lines by baseline (mergeUnitsByBaseline): overlapping boxes alone
// chain a margin note set on a different leading into the body lines beside
// it. The remaining characters (short runs, synthetic characters without a
// stream position) attach to an overlapping unit line or, failing that, are
// clustered by y-midpoint proximity (clusterNormalChars), with short-box
// punctuation attached to its nearest line (attachSmallChars) rather than
// being allowed to shift where lines split.
func groupLines(chars []pdfChar) []pdfLine {
	if len(chars) == 0 {
		return nil
	}
	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}

	units, loose := streamUnits(chars)
	groups := mergeUnitsByBaseline(units, medH)

	var normal, small []pdfChar
	for _, c := range loose {
		if c.top-c.bottom >= normalCharHeightRatio*medH {
			normal = append(normal, c)
		} else {
			small = append(small, c)
		}
	}
	if len(normal) == 0 && len(groups) == 0 {
		// Degenerate page (every character is "small") — cluster everything
		// so a page like this still produces output.
		normal, small = loose, nil
	}

	groups, normal = attachToUnitLines(groups, normal, medH)
	groups = append(groups, clusterNormalChars(normal, medH)...)
	groups = attachSmallChars(groups, small, medH)

	// Groups must run top to bottom before building lines; unit lines,
	// clustered lines, and attachSmallChars's new groups each come from
	// their own pass.
	sort.SliceStable(groups, func(i, j int) bool {
		return groupYMid(groups[i]) > groupYMid(groups[j])
	})

	lines := make([]pdfLine, len(groups))
	for i, g := range groups {
		lines[i] = buildLine(g)
	}
	return lines
}

// clusterNormalCharsOverlapMarginRatio is the small float-rounding tolerance
// applied to the vertical-interval overlap check in clusterNormalChars — not
// a line-spacing allowance like lineGroupYMidRatio (that ratio is deliberately
// too large for this check: applying it here reintroduces the false merge it
// was tuned to avoid, see the comment below).
const clusterNormalCharsOverlapMarginRatio = 0.05

// clusterNormalChars groups normal-height characters into lines by
// vertical-interval overlap rather than y-midpoint distance: sort by
// descending midpoint, then join a character to the line being built when
// its own [bottom, top] box overlaps the running envelope (min bottom, max
// top seen so far) of the line, within a small float-rounding margin.
//
// A single physical line set in a large or stylized font can span a wide
// range of y-midpoints — a cap-height letter's midpoint sits well above an
// x-height letter's, which sits above a descender's — and that spread can
// exceed lineGroupYMidRatio * medH when medH is calibrated off a much
// smaller body-text font sharing the page (the chapter-title fracturing in
// issue #1651). Cap, x-height, and descender glyphs on one baseline all
// still overlap each other's box near the baseline/x-height band regardless
// of font size, so overlap keeps them together where midpoint distance
// would not. Genuinely separate lines set with normal leading still don't
// overlap, so they still split — this only requires actual box overlap, not
// lineGroupYMidRatio * medH of slack the way attachSmallChars allows for
// small punctuation: that much tolerance here would let lines separated by
// a narrow but real gap merge into one.
func clusterNormalChars(chars []pdfChar, medH float64) [][]pdfChar {
	sorted := make([]pdfChar, len(chars))
	copy(sorted, chars)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].yMid() > sorted[j].yMid()
	})

	var groups [][]pdfChar
	var group []pdfChar
	var envTop, envBottom float64
	margin := clusterNormalCharsOverlapMarginRatio * medH

	flush := func() {
		if len(group) == 0 {
			return
		}
		groups = append(groups, group)
		group = nil
	}

	for _, c := range sorted {
		if len(group) > 0 && (c.bottom > envTop+margin || c.top < envBottom-margin) {
			flush()
		}
		if len(group) == 0 {
			envTop, envBottom = c.top, c.bottom
		} else {
			envTop = max(envTop, c.top)
			envBottom = min(envBottom, c.bottom)
		}
		group = append(group, c)
	}
	flush()

	return groups
}

// attachSmallChars assigns each short-box character to a group by
// vertical-interval overlap, not by distance from a single point: a small
// character joins the group whose [bottom, top] envelope — computed once
// from that group's normal-height characters, before any small characters
// are attached, so it can't grow across attachments — overlaps the
// character's own [bottom, top] box within lineGroupYMidRatio * medH.
// Overlap tolerates punctuation sitting on either side of the baseline
// (comma low, apostrophe high); among multiple overlapping groups the one
// whose y-midpoint is closest wins. A small character with no overlapping
// group starts its own group, so a page of pure punctuation still produces
// output.
func attachSmallChars(groups [][]pdfChar, small []pdfChar, medH float64) [][]pdfChar {
	margin := lineGroupYMidRatio * medH

	envelopes := make([]pdfLine, len(groups))
	for i, g := range groups {
		envelopes[i] = buildLine(append([]pdfChar(nil), g...))
	}

	for _, c := range small {
		best := -1
		var bestDist float64

		for i, env := range envelopes {
			if c.bottom > env.top+margin || c.top < env.bottom-margin {
				continue
			}
			dist := groupYMid(groups[i]) - c.yMid()
			if dist < 0 {
				dist = -dist
			}
			if best == -1 || dist < bestDist {
				best = i
				bestDist = dist
			}
		}

		if best == -1 {
			groups = append(groups, []pdfChar{c})
			continue
		}
		groups[best] = append(groups[best], c)
	}

	return groups
}

// groupYMid returns the average y-midpoint of a group of characters.
func groupYMid(g []pdfChar) float64 {
	var sum float64
	for _, c := range g {
		sum += c.yMid()
	}
	return sum / float64(len(g))
}

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
