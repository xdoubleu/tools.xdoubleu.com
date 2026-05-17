package templates

import (
	"context"
	"fmt"
	"html"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//nolint:gochecknoglobals // package-level config set once at startup
var (
	globalRelease       = "dev"
	globalCopyrightYear = time.Now().Format("2006")
)

// SetConfig sets package-level values used by layout templates.
// Call this once at application startup before serving requests.
func SetConfig(release string) {
	globalRelease = release
	globalCopyrightYear = time.Now().Format("2006")
}

func copyrightYear() string { return globalCopyrightYear }
func release() string       { return globalRelease }

// RenderError renders the shared error page with the given HTTP status and message.
func RenderError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = ErrorPage(
		status,
		http.StatusText(status),
		message,
	).Render(context.Background(), w)
}

const eighths = 8

//nolint:gochecknoglobals //package-level lookup table, not mutable state
var fractionSymbols = map[int]string{
	0: "",
	1: "⅛",
	2: "¼",
	3: "⅜",
	4: "½",
	5: "⅝",
	6: "¾",
	7: "⅞",
}

// ToFraction converts a float64 to a Unicode cooking fraction string (nearest 1/8th).
func ToFraction(f float64) string {
	if f <= 0 {
		return "0"
	}
	whole := int(math.Floor(f))
	nearest := int(math.Round((f - float64(whole)) * eighths))
	if nearest == eighths {
		whole++
		nearest = 0
	}
	fracStr := fractionSymbols[nearest]
	if whole == 0 {
		if fracStr == "" {
			return "0"
		}
		return fracStr
	}
	return fmt.Sprintf("%d%s", whole, fracStr)
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
