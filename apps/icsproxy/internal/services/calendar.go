package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
)

type CalendarService struct {
	logger *slog.Logger
	repo   *repositories.CalendarRepository
}

// ============================================================
// Persistence (unchanged)
// ============================================================

func (s *CalendarService) SaveConfig(
	ctx context.Context,
	cfg models.FilterConfig,
) error {
	return s.repo.UpsertFilterConfig(ctx, cfg)
}

func (s *CalendarService) LoadConfig(
	ctx context.Context,
	token string,
) (models.FilterConfig, bool) {
	return s.repo.GetFilterConfig(ctx, token)
}

func (s *CalendarService) ListConfigs(
	ctx context.Context,
) ([]models.FilterConfig, error) {
	return s.repo.ListFilterConfigs(ctx)
}

func (s *CalendarService) ListConfigSummaries(
	ctx context.Context,
) ([]repositories.FilterSummary, error) {
	return s.repo.ListFilterSummaries(ctx)
}
func (s *CalendarService) DeleteConfig(ctx context.Context, token string) error {
	return s.repo.DeleteFilterConfig(ctx, token)
}

// ============================================================
// Fetch (unchanged)
// ============================================================

func (s *CalendarService) FetchICS(ctx context.Context, url string) ([]byte, error) {
	parsed, err := neturl.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("invalid calendar url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "localhost" ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "172.") {
		return nil, fmt.Errorf("private hosts are not allowed: %s", host)
	}

	//nolint:mnd //10s is a reasonable default for calendar fetches
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "tools.xdoubleu.com-icsproxy/1.0")
	req.Header.Set("Accept", "text/calendar")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 from calendar: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// ============================================================
// SMART NAME NORMALIZATION
// ============================================================

var dateSuffixRegex = regexp.MustCompile(
	`(?i)\s*\d{1,2}[/-]\d{1,2}[/-]\d{2,4}\s*-\s*\d{1,2}[/-]\d{1,2}[/-]\d{2,4}$`,
)

var versionSuffixRegex = regexp.MustCompile(
	`(?i)\s*-\s*(v?\d+(\.\d+)+)$`,
)

// Strip trailing dates or versions.
func normalizeSummary(summary string) string {
	base := strings.TrimSpace(summary)
	base = dateSuffixRegex.ReplaceAllString(base, "")
	base = versionSuffixRegex.ReplaceAllString(base, "")
	return strings.TrimSpace(base)
}

// This is the TRUE grouping key.
func makeSeriesKey(summary string) string {
	return normalizeSummary(summary)
}

// ============================================================
// Formatting helper
// ============================================================

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

// ============================================================
// 🔥 KEY FIX: GROUP EVENTS FOR PREVIEW 🔥
// ============================================================

func (s *CalendarService) ExtractEvents(data []byte) ([]models.EventInfo, error) {
	cal, err := ics.ParseCalendar(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	type rawEvent struct {
		info models.EventInfo
	}

	grouped := map[string]rawEvent{} // key = normalized name

	for _, comp := range cal.Components {
		ev, ok := comp.(*ics.VEvent)
		if !ok {
			continue
		}

		// Hide modified instances from preview
		if ev.GetProperty("RECURRENCE-ID") != nil {
			continue
		}

		rawSummary := ev.GetProperty("SUMMARY").Value
		baseName := normalizeSummary(rawSummary)
		baseKey := makeSeriesKey(rawSummary)

		startRaw := ev.GetProperty("DTSTART").Value
		endRaw := ev.GetProperty("DTEND").Value
		uid := ev.GetProperty("UID").Value

		// Only set RRule if this event truly has one
		var rrule string
		if p := ev.GetProperty("RRULE"); p != nil {
			rrule = p.Value
		}

		// Keep first occurrence per group
		if _, exists := grouped[baseKey]; !exists {
			grouped[baseKey] = rawEvent{
				info: models.EventInfo{
					UID:       uid,
					Summary:   baseName,
					StartRaw:  startRaw,
					EndRaw:    endRaw,
					StartNice: formatICSTime(startRaw),
					EndNice:   formatICSTime(endRaw),
					RRule:     rrule,
					SeriesKey: baseKey,
				},
			}
		}
	}

	// ---- NEW: convert map to slice AND sort by date ----
	var events []models.EventInfo
	for _, v := range grouped {
		events = append(events, v.info)
	}

	sort.Slice(events, func(i, j int) bool {
		ti, _ := parseICSTime(events[i].StartRaw)
		tj, _ := parseICSTime(events[j].StartRaw)
		return ti.Before(tj)
	})

	return events, nil
}

// ============================================================
// APPLY FILTER (works on ALL instances)
// ============================================================

func (s *CalendarService) ApplyFilter(
	data []byte,
	cfg models.FilterConfig,
) ([]byte, error) {
	cal, err := ics.ParseCalendar(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	events, err := s.ExtractEvents(data)
	if err != nil {
		return nil, err
	}

	holidayWindow, hasHoliday := s.findHolidayWindow(events, cfg)

	var newComponents []ics.Component

	for _, comp := range cal.Components {
		ev, ok := comp.(*ics.VEvent)
		if !ok {
			newComponents = append(newComponents, comp)
			continue
		}

		if s.shouldHideEvent(ev, cfg, holidayWindow, hasHoliday) {
			continue
		}

		newComponents = append(newComponents, ev)
	}

	cal.Components = newComponents
	return []byte(cal.Serialize()), nil
}

// ============================================================
// HELPERS
// ============================================================

type holidayWindow struct {
	start time.Time
	end   time.Time
}

func (s *CalendarService) findHolidayWindow(
	events []models.EventInfo,
	cfg models.FilterConfig,
) (holidayWindow, bool) {
	var w holidayWindow

	for _, ev := range events {
		for _, h := range cfg.HolidayUIDs {
			if ev.UID != h {
				continue
			}

			start, err1 := parseICSTime(ev.StartRaw)
			end, err2 := parseICSTime(ev.EndRaw)
			if err1 != nil || err2 != nil {
				continue
			}
			w = holidayWindow{start: start, end: end}
			return w, true
		}
	}
	return w, false
}

func (s *CalendarService) shouldHideEvent(
	ev *ics.VEvent,
	cfg models.FilterConfig,
	w holidayWindow,
	hasHoliday bool,
) bool {
	rawSummary := ev.GetProperty("SUMMARY").Value
	baseKey := makeSeriesKey(rawSummary)
	uid := ev.GetProperty("UID").Value

	// 1) Explicit UID hide
	if s.isExplicitlyHidden(uid, cfg) {
		return true
	}

	// 2) Hide whole FUZZY series
	if cfg.HideSeries[baseKey] {
		return true
	}

	// 3) Holiday window applies to ALL matching names
	if hasHoliday && s.overlapsHoliday(ev, w) {
		return true
	}

	return false
}

func (s *CalendarService) isExplicitlyHidden(uid string, cfg models.FilterConfig) bool {
	for _, h := range cfg.HideEventUIDs {
		if h == uid {
			return true
		}
	}
	return false
}

func (s *CalendarService) overlapsHoliday(ev *ics.VEvent, w holidayWindow) bool {
	startProp := ev.GetProperty("DTSTART")
	endProp := ev.GetProperty("DTEND")

	evStart, err1 := parseICSTimeWithTZID(startProp)
	evEnd, err2 := parseICSTimeWithTZID(endProp)
	if err1 != nil || err2 != nil {
		return false
	}
	return evStart.Before(w.end) && evEnd.After(w.start)
}

// ============================================================
// TZ PARSERS (unchanged)
// ============================================================

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
		loc, err := time.LoadLocation(tzid[0])
		if err == nil {
			var t time.Time
			if t, err = time.ParseInLocation("20060102T150405", raw, loc); err == nil {
				return t, nil
			}
		}
	}
	return parseICSTime(raw)
}
