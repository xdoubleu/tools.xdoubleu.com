package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

var dateSuffixRegex = regexp.MustCompile(
	`(?i)\s*\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\s*-\s*\d{1,2}[/-]\d{1,2}[/-]\d{2,4}$`,
)

var versionSuffixRegex = regexp.MustCompile(
	`(?i)\s*-\s*(v?\d+(\.\d+)+)$`,
)

func normalizeSummary(summary string) string {
	base := strings.TrimSpace(summary)
	base = dateSuffixRegex.ReplaceAllString(base, "")
	base = versionSuffixRegex.ReplaceAllString(base, "")
	return strings.TrimSpace(base)
}

func makeSeriesKey(summary string) string {
	return normalizeSummary(summary)
}

func formatICSTime(raw string) string {
	raw = strings.ReplaceAll(raw, "DTSTART;VALUE=DATE:", "")
	raw = strings.ReplaceAll(raw, "DTEND;VALUE=DATE:", "")

	if t, err := time.Parse("20060102", raw); err == nil {
		return t.Format("2 Jan 2006")
	}
	if t, err := time.Parse("20060102T150405", raw); err == nil {
		return t.Format("2 Jan 2006, 15:04")
	}
	return raw
}

func parseICSTime(raw string) (time.Time, error) {
	raw = strings.ReplaceAll(raw, "DTSTART;VALUE=DATE:", "")
	raw = strings.ReplaceAll(raw, "DTEND;VALUE=DATE:", "")

	if t, err := time.Parse("20060102T150405-0700", raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("20060102T150405Z", raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("20060102T150405", raw); err == nil {
		return t.In(time.Local), nil
	}
	if t, err := time.Parse("20060102", raw); err == nil {
		return t.In(time.Local), nil
	}

	return time.Time{}, fmt.Errorf("cannot parse ICS time: %s", raw)
}

func parseICSTimeWithTZID(p *ics.IANAProperty) (time.Time, error) {
	raw := p.Value

	if tzid, ok := p.ICalParameters["TZID"]; ok && len(tzid) > 0 {
		loc, err := time.LoadLocation(normalizeTZID(tzid[0]))
		if err == nil {
			var t time.Time
			if t, err = time.Parse("20060102T150405", raw); err == nil {
				return time.Date(
					t.Year(),
					t.Month(),
					t.Day(),
					t.Hour(),
					t.Minute(),
					t.Second(),
					0,
					loc,
				), nil
			}
		}
	}

	return parseICSTime(raw)
}

const europeBrussels = "Europe/Brussels"

func normalizeTZID(tz string) string {
	switch tz {
	case "Romance Standard Time":
		return europeBrussels
	case "Central Europe Standard Time":
		return "Europe/Berlin"
	case "W. Europe Standard Time":
		return "Europe/Amsterdam"
	case "GMT Standard Time":
		return "Europe/London"
	default:
		return tz
	}
}
