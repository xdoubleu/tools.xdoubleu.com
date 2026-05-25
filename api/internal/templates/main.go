package templates

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SetConfig is a no-op retained for API compatibility.
func SetConfig(_ string) {}

// RenderError renders an HTTP error response with the given status and message.
func RenderError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}

type fracEntry struct {
	val    float64
	symbol string
}

//nolint:gochecknoglobals //package-level lookup table, not mutable state
var commonFractions = []fracEntry{
	{0.0, ""},
	{1.0 / 8, "⅛"},
	{1.0 / 4, "¼"},
	{1.0 / 3, "⅓"},
	{3.0 / 8, "⅜"},
	{1.0 / 2, "½"},
	{5.0 / 8, "⅝"},
	{2.0 / 3, "⅔"},
	{3.0 / 4, "¾"},
	{7.0 / 8, "⅞"},
	{1.0, ""},
}

// ToFraction converts a float64 to a Unicode cooking fraction string.
func ToFraction(f float64) string {
	if f <= 0 {
		return "0"
	}
	whole := int(math.Floor(f))
	frac := f - float64(whole)

	bestDiff := math.MaxFloat64
	bestIdx := 0
	for i, cf := range commonFractions {
		if diff := math.Abs(frac - cf.val); diff <= bestDiff {
			bestDiff = diff
			bestIdx = i
		}
	}
	symbol := commonFractions[bestIdx].symbol
	if bestIdx == len(commonFractions)-1 {
		whole++
		symbol = ""
	}

	if whole == 0 {
		if symbol == "" {
			return "0"
		}
		return symbol
	}
	if symbol == "" {
		return fmt.Sprintf("%d", whole)
	}
	return fmt.Sprintf("%d%s", whole, symbol)
}

var mdLinkRE = regexp.MustCompile(`\[([^\]]+)\]\(((?:https?://)?[^\s)]+)\)`)

// HasMdLink reports whether s contains a markdown link.
func HasMdLink(s string) bool { return mdLinkRE.MatchString(s) }

// RenderTitleLinks replaces [title](url) markdown links in s with HTML <a> tags.
func RenderTitleLinks(s string) string {
	var b strings.Builder
	last := 0
	for _, m := range mdLinkRE.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(html.EscapeString(s[last:m[0]]))
		title := html.EscapeString(s[m[2]:m[3]])
		u := s[m[4]:m[5]]
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			u = "https://" + u
		}
		rawURL := html.EscapeString(u)
		fmt.Fprintf(&b,
			`<a href="%s" target="_blank" rel="noopener noreferrer"`+
				` onclick="event.stopPropagation()"`+
				` class="text-decoration-underline text-body task-title-link">%s</a>`,
			rawURL, title,
		)
		last = m[1]
	}
	b.WriteString(html.EscapeString(s[last:]))
	return b.String()
}

var recurOrdinals = map[int]string{ //nolint:gochecknoglobals // read-only lookup table
	1: "first", 2: "second", 3: "third", 4: "fourth", 5: "fifth", -1: "last",
}

// RecurInputDisplay converts a stored recur rule (e.g. "weekday:4") to its
// human-readable form (e.g. "every thursday").
func RecurInputDisplay(rule string) string {
	if rule == "" {
		return ""
	}
	parts := strings.Split(rule, ":")
	switch parts[0] {
	case "days":
		if len(parts) == 2 { //nolint:mnd // 2-part rule is self-documenting
			return "every " + parts[1] + " days"
		}
	case "weekday":
		if len(parts) == 2 { //nolint:mnd // 2-part rule is self-documenting
			if w, err := strconv.Atoi(parts[1]); err == nil && w >= 0 &&
				w <= 6 {
				return "every " + strings.ToLower(time.Weekday(w).String())
			}
		}
	case "monthweekday":
		if len(parts) == 3 { //nolint:mnd // 3-part rule is self-documenting
			o, err1 := strconv.Atoi(parts[1])
			w, err2 := strconv.Atoi(parts[2])
			if err1 == nil && err2 == nil && w >= 0 &&
				w <= 6 {
				if name, ok := recurOrdinals[o]; ok {
					return "every " + name + " " +
						strings.ToLower(time.Weekday(w).String())
				}
			}
		}
	}
	return rule
}

// HumanDate converts a *time.Time to a human-readable relative day string.
func HumanDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	now := time.Now()
	today := time.Date(
		now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local,
	)
	d := time.Date(
		t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local,
	)
	//nolint:mnd // 24h/day, 7 days/week are fixed constants
	days := int(math.Round(d.Sub(today).Hours() / 24))
	switch days {
	case -1:
		return "Yesterday"
	case 0:
		return "Today"
	case 1:
		return "Tomorrow"
	default:
		if days > 1 && days < 7 {
			return d.Format("Mon")
		}
		return d.Format("2 Jan")
	}
}

// IsOverdue reports whether t is before today (i.e. past due).
func IsOverdue(t *time.Time) bool {
	if t == nil {
		return false
	}
	now := time.Now()
	today := time.Date(
		now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local,
	)
	d := time.Date(
		t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local,
	)
	return d.Before(today)
}

// DescFirstLine returns the first line of s, trimmed of whitespace.
func DescFirstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}
