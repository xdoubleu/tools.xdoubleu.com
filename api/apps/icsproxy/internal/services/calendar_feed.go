package services

import (
	"bytes"
	"context"
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
	oneDay               = 24 * time.Hour
)

func (s *CalendarService) FetchICS(ctx context.Context, url string) ([]byte, error) {
	parsed, err := neturl.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("invalid calendar url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", parsed.Scheme)
	}

	span := sentry.StartSpan(
		ctx,
		"http.client",
		sentry.WithDescription("FetchICS "+url),
	)
	defer span.Finish()

	client := &http.Client{Timeout: calendarFetchTimeout}
	req, _ := http.NewRequestWithContext(span.Context(), http.MethodGet, url, nil)
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

		rawSummary := ev.GetProperty("SUMMARY").Value
		baseName := normalizeSummary(rawSummary)
		baseKey := makeSeriesKey(rawSummary)

		startRaw := ev.GetProperty("DTSTART").Value
		endRaw := ev.GetProperty("DTEND").Value
		uid := ev.GetProperty("UID").Value

		var rrule string
		if p := ev.GetProperty("RRULE"); p != nil {
			rrule = p.Value
		}

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
