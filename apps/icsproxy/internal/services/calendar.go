package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
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
// Persistence
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

func (s *CalendarService) DeleteConfig(
	ctx context.Context,
	token string,
) error {
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

	//nolint:mnd // It's clearer to have the timeout here inlined
	client := &http.Client{Timeout: 10 * time.Second}

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
// Formatting helpers
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
// Parsing — **UPDATED FOR YOUR REQUIREMENT**
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

		// -----------------------------------------------------------
		// 🔥 YOUR REQUIREMENT: hide modified instances from preview
		// -----------------------------------------------------------
		if ev.GetProperty("RECURRENCE-ID") != nil {
			s.logger.Debug(
				"Skipping modified instance in preview",
				"uid", ev.GetProperty("UID").Value,
			)
			continue
		}
		// -----------------------------------------------------------

		startRaw := ev.GetProperty("DTSTART").Value
		endRaw := ev.GetProperty("DTEND").Value
		uid := ev.GetProperty("UID").Value
		summary := ev.GetProperty("SUMMARY").Value

		rrule := ""
		if p := ev.GetProperty("RRULE"); p != nil {
			rrule = p.Value
		}

		// RDATE-only series should still count as recurring
		if rrule == "" && ev.GetProperty("RDATE") != nil {
			rrule = "RDATE"
		}

		// Stable key for the series (important for filtering)
		seriesKey := summary + "|" + uid

		events = append(events, models.EventInfo{
			UID:       uid,
			Summary:   summary,
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
// Filtering — IN-PLACE (still hides RECURRENCE-ID instances)
// ============================================================

func (s *CalendarService) ApplyFilter(
	data []byte,
	cfg models.FilterConfig,
) ([]byte, error) {
	cal, err := ics.ParseCalendar(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// NOTE: ExtractEvents no longer includes RECURRENCE-ID events,
	// but that's OK — we only need it to find holidays + build keys.

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

		// IMPORTANT:
		// Even though RECURRENCE-ID events were hidden from the preview,
		// they are still passed through shouldHideEvent here and will be
		// removed when their series is hidden.

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

	// Series key must match ExtractEvents logic
	seriesKey := summary + "|" + uid

	if s.isExplicitlyHidden(uid, cfg) {
		return true
	}

	// 🔥 This is why your modified instances STILL get hidden:
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
