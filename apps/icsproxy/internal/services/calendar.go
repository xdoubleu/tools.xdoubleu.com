package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
)

type CalendarService struct {
	repo *repositories.CalendarRepository
}

// ============================================================
// Persistence (UPDATED TO MATCH YOUR NEW REPO)
// ============================================================

// Save or update a filter config (upsert).
func (s *CalendarService) SaveConfig(
	ctx context.Context,
	cfg models.FilterConfig,
) error {
	return s.repo.UpsertFilterConfig(ctx, cfg)
}

// Load a single config by token.
func (s *CalendarService) LoadConfig(
	ctx context.Context,
	token string,
) (models.FilterConfig, bool) {
	return s.repo.GetFilterConfig(ctx, token)
}

// List all configs (full objects).
func (s *CalendarService) ListConfigs(
	ctx context.Context,
) ([]models.FilterConfig, error) {
	return s.repo.ListFilterConfigs(ctx)
}

// List lightweight summaries for homepage.
func (s *CalendarService) ListConfigSummaries(
	ctx context.Context,
) ([]repositories.FilterSummary, error) {
	return s.repo.ListFilterSummaries(ctx)
}

// Delete a config.
func (s *CalendarService) DeleteConfig(
	ctx context.Context,
	token string,
) error {
	return s.repo.DeleteFilterConfig(ctx, token)
}

// ============================================================
// Fetch (gosec-safe + SSRF-safe)
// ============================================================

func (s *CalendarService) FetchICS(ctx context.Context, url string) ([]byte, error) {
	parsed, err := neturl.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("invalid calendar url: %w", err)
	}

	// Allow only HTTP/HTTPS
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	// Basic SSRF protection
	host := parsed.Hostname()
	if host == "localhost" ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "172.") {
		return nil, fmt.Errorf("private hosts are not allowed: %s", host)
	}

	client := &http.Client{
		//nolint:mnd // Set a reasonable timeout for fetching external calendars
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

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
// Formatting helpers (for preview only)
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
// Parsing (for preview)
// ============================================================

func (s *CalendarService) ExtractEvents(data []byte) ([]models.EventInfo, error) {
	cal, err := ics.ParseCalendar(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	var events []models.EventInfo

	for _, comp := range cal.Components {
		ev, ok := comp.(*ics.VEvent)
		if !ok {
			continue
		}

		startRaw := ev.GetProperty("DTSTART").Value
		endRaw := ev.GetProperty("DTEND").Value

		rrule := ""
		if p := ev.GetProperty("RRULE"); p != nil {
			rrule = p.Value
		}

		seriesKey := ev.GetProperty("SUMMARY").Value + "|"
		if rrule != "" {
			seriesKey += rrule
		} else {
			seriesKey += "SINGLE"
		}

		events = append(events, models.EventInfo{
			UID:       ev.GetProperty("UID").Value,
			Summary:   ev.GetProperty("SUMMARY").Value,
			StartRaw:  startRaw,
			EndRaw:    endRaw,
			StartNice: formatICSTime(startRaw),
			EndNice:   formatICSTime(endRaw),
			RRule:     rrule,
			SeriesKey: seriesKey,
		})
	}

	return events, nil
}

// ============================================================
// Filtering — IN-PLACE (preserves all timezones)
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
// Helpers
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
	uid := ev.GetProperty("UID").Value
	summary := ev.GetProperty("SUMMARY").Value

	rrule := ""
	if p := ev.GetProperty("RRULE"); p != nil {
		rrule = p.Value
	}

	seriesKey := summary + "|" + func() string {
		if rrule != "" {
			return rrule
		}
		return "SINGLE"
	}()

	if s.isExplicitlyHidden(uid, cfg) {
		return true
	}

	if cfg.HideSeries[seriesKey] {
		return true
	}

	if hasHoliday && s.overlapsHoliday(ev, w) {
		return true
	}

	return false
}

func (s *CalendarService) isExplicitlyHidden(
	uid string,
	cfg models.FilterConfig,
) bool {
	for _, h := range cfg.HideEventUIDs {
		if h == uid {
			return true
		}
	}
	return false
}

func (s *CalendarService) overlapsHoliday(
	ev *ics.VEvent,
	w holidayWindow,
) bool {
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
// TZ-AWARE ICS TIME PARSERS
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
