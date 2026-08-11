package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"sort"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

const (
	calendarFetchTimeout = 10 * time.Second
	maxCalendarRedirects = 5
	// maxCalendarBytes bounds how much of an upstream calendar we buffer —
	// the api process is shared by every app, so an unbounded read is a
	// memory-exhaustion lever for anyone who can point a feed at a big file.
	maxCalendarBytes = int64(5 << 20) // 5 MiB
	oneDay           = 24 * time.Hour
)

var errCalendarTooLarge = errors.New("calendar too large")

// FetchICS retrieves the calendar at url.
//
// Source URLs are secrets (a private Outlook/Google ICS link is a bearer
// credential), so neither the span description nor any returned error may
// contain the full URL — errors here surface in logs and Sentry. The client
// itself refuses non-public addresses; see [safedial].
func (s *CalendarService) FetchICS(ctx context.Context, url string) ([]byte, error) {
	parsed, err := neturl.Parse(url)
	if err != nil {
		return nil, errors.New("invalid calendar url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	span := sentry.StartSpan(
		ctx,
		"http.client",
		sentry.WithDescription("FetchICS "+parsed.Host),
	)
	defer span.Finish()

	req, err := http.NewRequestWithContext(
		span.Context(), http.MethodGet, url, nil,
	)
	if err != nil {
		return nil, errors.New("invalid calendar url")
	}
	req.Header.Set("User-Agent", "tools.xdoubleu.com-icsproxy/1.0")
	req.Header.Set("Accept", "text/calendar")

	resp, err := s.client.Do(req)
	if err != nil {
		// Deliberately not wrapped: *url.Error carries the full URL.
		return nil, fmt.Errorf("fetching calendar from %s failed", parsed.Host)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 from calendar: %d", resp.StatusCode)
	}

	// Read one byte past the cap to tell exactly-at-cap from over.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCalendarBytes+1))
	if err != nil {
		return nil, errors.New("reading calendar failed")
	}
	if int64(len(data)) > maxCalendarBytes {
		return nil, fmt.Errorf(
			"%w: over %d bytes",
			errCalendarTooLarge,
			maxCalendarBytes,
		)
	}

	return data, nil
}

// PreviewEvents fetches the calendar at sourceURL and extracts its events,
// without persisting anything. Shared by the PreviewEvents RPC and
// GetConfigWithEvents.
func (s *CalendarService) PreviewEvents(
	ctx context.Context,
	sourceURL string,
) ([]models.EventInfo, error) {
	data, err := s.FetchICS(ctx, sourceURL)
	if err != nil {
		return nil, err
	}
	return s.ExtractEvents(ctx, data)
}

func (s *CalendarService) ExtractEvents(
	ctx context.Context,
	data []byte,
) ([]models.EventInfo, error) {
	span := sentry.StartSpan(ctx, "function", sentry.WithDescription("ExtractEvents"))
	defer span.Finish()

	all, err := s.extractAllEvents(data)
	if err != nil {
		return nil, err
	}

	// Group by base name for UI
	grouped := map[string]models.EventInfo{}

	for _, ev := range all {
		// Hide modified instances from preview
		if ev.HasRecurrenceID {
			continue
		}

		if _, exists := grouped[ev.SeriesKey]; !exists {
			grouped[ev.SeriesKey] = ev
		}
	}

	var events []models.EventInfo
	for _, v := range grouped {
		events = append(events, v)
	}

	sort.Slice(events, func(i, j int) bool {
		ti, _ := parseICSTime(events[i].StartRaw)
		tj, _ := parseICSTime(events[j].StartRaw)
		return ti.Before(tj)
	})

	return events, nil
}

// propValue reads an event property, returning "" when it is absent.
// golang-ical returns a nil *IANAProperty for a missing property, and an
// upstream calendar is free to omit even UID/DTSTART — dereferencing that
// directly turned any malformed VEVENT into a panic.
func propValue(ev *ics.VEvent, name ics.ComponentProperty) string {
	p := ev.GetProperty(name)
	if p == nil {
		return ""
	}
	return p.Value
}

func (s *CalendarService) extractAllEvents(data []byte) ([]models.EventInfo, error) {
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

		rawSummary := propValue(ev, "SUMMARY")
		baseName := normalizeSummary(rawSummary)
		baseKey := makeSeriesKey(rawSummary)

		startRaw := propValue(ev, "DTSTART")
		endRaw := propValue(ev, "DTEND")
		uid := propValue(ev, "UID")
		rrule := propValue(ev, "RRULE")

		hasRecID := ev.GetProperty("RECURRENCE-ID") != nil

		events = append(events, models.EventInfo{
			UID:             uid,
			Summary:         baseName,
			StartRaw:        startRaw,
			EndRaw:          endRaw,
			StartNice:       formatICSTime(startRaw),
			EndNice:         formatICSTime(endRaw),
			RRule:           rrule,
			SeriesKey:       baseKey,
			HasRecurrenceID: hasRecID,
		})
	}

	return events, nil
}
