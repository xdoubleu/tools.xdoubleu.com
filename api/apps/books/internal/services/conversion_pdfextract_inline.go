package services

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	// supRaiseRatio: a digit whose bottom sits this share of the line's
	// character height above the baseline is a superscript (a note mark).
	supRaiseRatio = 0.25
	// boldBlockShare: a block with at least this share of bold characters
	// is set in bold as a whole (a note, a heading); only bold runs inside
	// otherwise regular text get <strong>.
	boldBlockShare = 0.5
)

// inlineStyle is the inline markup a character renders with.
type inlineStyle struct {
	em, strong, sup bool
}

func isItalicFont(font string) bool {
	f := strings.ToLower(font)
	return strings.Contains(f, "italic") || strings.Contains(f, "oblique")
}

func isBoldFont(font string) bool {
	f := strings.ToLower(font)
	for _, w := range []string{"bold", "semibold", "black", "heavy", "demi"} {
		if strings.Contains(f, w) {
			return true
		}
	}
	return false
}

func charStyle(c pdfChar, baseline, medH float64, allowBold bool) inlineStyle {
	return inlineStyle{
		em:     isItalicFont(c.font),
		strong: allowBold && isBoldFont(c.font),
		sup: unicode.IsDigit(firstRune(c.text)) && medH > 0 &&
			c.bottom-baseline > supRaiseRatio*medH,
	}
}

func (s inlineStyle) open() string {
	var b strings.Builder
	if s.strong {
		b.WriteString("<strong>")
	}
	if s.em {
		b.WriteString("<em>")
	}
	if s.sup {
		b.WriteString("<sup>")
	}
	return b.String()
}

func (s inlineStyle) close() string {
	var b strings.Builder
	if s.sup {
		b.WriteString("</sup>")
	}
	if s.em {
		b.WriteString("</em>")
	}
	if s.strong {
		b.WriteString("</strong>")
	}
	return b.String()
}

// lineHTML renders a line's characters as escaped text with inline markup,
// spacing them as joinChars does.
func lineHTML(l pdfLine, allowBold bool) string {
	chars := l.chars
	if len(chars) == 0 {
		return escapeXMLText(l.text)
	}
	medH := medianCharHeight(chars)
	if medH <= 0 {
		medH = 1
	}
	ratio := l.spaceRatio
	if ratio <= 0 {
		ratio = lineSpaceGapRatio
	}
	baseline := unitBaseline(chars)

	var b strings.Builder
	var cur inlineStyle
	for i, c := range chars {
		style := charStyle(c, baseline, l.medianCharHeight, allowBold)
		space := i > 0 && needsRunBoundarySpace(chars[i-1], c, medH, ratio)
		if style != cur {
			b.WriteString(cur.close())
			if space {
				b.WriteByte(' ')
			}
			b.WriteString(style.open())
			cur = style
		} else if space {
			b.WriteByte(' ')
		}
		b.WriteString(escapeXMLText(c.text))
	}
	b.WriteString(cur.close())
	return b.String()
}

// paragraphHTML renders a paragraph's lines with inline markup, joining
// them as joinLinesWithHyphenation does.
func paragraphHTML(lines []pdfLine) string {
	var bold, total int
	for _, l := range lines {
		for _, c := range l.chars {
			total++
			if isBoldFont(c.font) {
				bold++
			}
		}
	}
	allowBold := float64(bold) < boldBlockShare*float64(total)

	var b strings.Builder
	text := ""
	for i, l := range lines {
		h := lineHTML(l, allowBold)
		if i == 0 {
			b.WriteString(h)
			text = l.text
			continue
		}
		joined := b.String()
		if strings.HasSuffix(text, "-") && startsLower(l.text) {
			joined = removeLastHyphen(joined)
		} else {
			joined += " "
		}
		b.Reset()
		b.WriteString(joined + h)
		text = l.text
	}
	return adjacentRuns.Replace(b.String())
}

// adjacentRuns merges a run closed and reopened with the same tag, as at a
// line break inside it.
var adjacentRuns = strings.NewReplacer(
	"</em><em>", "", "</em> <em>", " ",
	"</strong><strong>", "", "</strong> <strong>", " ",
	"</sup><sup>", "", "</sup> <sup>", " ",
)

// removeLastHyphen drops the last "-" of an HTML fragment whose text ends
// in a line-end hyphen (closing tags may follow it).
func removeLastHyphen(h string) string {
	i := strings.LastIndex(h, "-")
	if i < 0 {
		return h
	}
	return h[:i] + h[i+1:]
}

// joinContinuationHTML joins the HTML of a paragraph's two halves the way
// joinContinuation joins their text.
func joinContinuationHTML(aText, a, bText, b string) string {
	if strings.HasSuffix(aText, "-") && startsLower(bText) {
		return removeLastHyphen(a) + b
	}
	return a + " " + b
}

// leadingMarkerRe matches a bullet at the start of HTML, after any opening
// tags.
var leadingMarkerRe = regexp.MustCompile(`^((?:<[^/>][^>]*>)*)[•◦▪‣–]\s+`)

// stripLeadingMarker removes a list item's bullet from its HTML.
func stripLeadingMarker(h string) string {
	return leadingMarkerRe.ReplaceAllString(h, "$1")
}
