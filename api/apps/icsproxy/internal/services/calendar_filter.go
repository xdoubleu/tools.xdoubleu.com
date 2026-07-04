package services

import (
	"bytes"
	"context"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/getsentry/sentry-go"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
)

//nolint:gocognit,funlen //it works stfu
func (s *CalendarService) ApplyFilter(
	ctx context.Context,
	data []byte,
	cfg models.FilterConfig,
) ([]byte, error) {
	span := sentry.StartSpan(ctx, "function", sentry.WithDescription("ApplyFilter"))
	defer span.Finish()
	cal, err := ics.ParseCalendar(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	events, err := s.extractAllEvents(data)
	if err != nil {
		return nil, err
	}

	holidayWindows := s.findHolidayWindows(events, cfg)
	hasHoliday := len(holidayWindows) > 0

	var newComponents []ics.Component

OUTER:
	for _, comp := range cal.Components {
		ev, ok := comp.(*ics.VEvent)
		if !ok {
			newComponents = append(newComponents, comp)
			continue
		}

		uid := ev.GetProperty("UID").Value
		rawSummary := ev.GetProperty("SUMMARY").Value
		baseKey := makeSeriesKey(rawSummary)

		// -------------------------------------------------------
		// RULE A — NEVER hide holiday events themselves
		// -------------------------------------------------------
		for _, h := range cfg.HolidayUIDs {
			if uid == h {
				newComponents = append(newComponents, ev)
				continue
			}
		}

		// -------------------------------------------------------
		// RULE B — NON-RECURRING events → delete if needed
		// -------------------------------------------------------
		if ev.GetProperty("RRULE") == nil {
			if s.shouldHideEvent(ev, cfg, holidayWindows, hasHoliday) {
				continue
			}
			newComponents = append(newComponents, ev)
			continue
		}

		// -------------------------------------------------------
		// RULE C — Hidden RECURRING events
		// -------------------------------------------------------
		for hideKey := range cfg.HideSeries {
			if hideKey == baseKey {
				// ❌ remove the entire recurring series
				continue OUTER
			}
		}

		// -------------------------------------------------------
		// RULE D — RECURRING events → ADD PRECISE EXDATE LIST
		// -------------------------------------------------------
		if hasHoliday {
			// Get the SERIES DTSTART time-of-day (needed for correct EXDATE)
			seriesStartProp := ev.GetProperty("DTSTART")
			var seriesStart time.Time
			seriesStart, err = parseICSTimeWithTZID(seriesStartProp)
			if err != nil {
				newComponents = append(newComponents, ev)
				continue
			}

			for _, w := range holidayWindows {
				// Build list of all excluded occurrences
				var exdates []string

				d := w.start
				for !d.After(w.end) {
					// IMPORTANT: keep the SAME time-of-day as the recurring event
					occurrence := time.Date(
						d.Year(),
						d.Month(),
						d.Day(),
						seriesStart.Hour(),
						seriesStart.Minute(),
						seriesStart.Second(),
						0,
						seriesStart.Location(),
					)

					d = d.Add(oneDay)

					if !occurrence.After(w.start) {
						continue
					}

					exdates = append(
						exdates,
						occurrence.UTC().Format("20060102T150405"),
					)
				}

				// Add a single EXDATE with comma-separated values
				if len(exdates) > 0 {
					ev.AddProperty("EXDATE", strings.Join(exdates, ","))
				}
			}
		}

		newComponents = append(newComponents, ev)
	}

	cal.Components = newComponents
	return []byte(cal.Serialize()), nil
}

type holidayWindow struct {
	start time.Time
	end   time.Time
}

func (s *CalendarService) findHolidayWindows(
	events []models.EventInfo,
	cfg models.FilterConfig,
) []holidayWindow {
	var windows []holidayWindow
	holidayNames := map[string]bool{}

	// Collect base names marked as holiday
	for _, ev := range events {
		for _, h := range cfg.HolidayUIDs {
			if ev.UID == h {
				holidayNames[ev.SeriesKey] = true
			}
		}
	}

	// Any event with that base name becomes a holiday window
	for _, ev := range events {
		if !holidayNames[ev.SeriesKey] {
			continue
		}

		start, err1 := parseICSTime(ev.StartRaw)
		end, err2 := parseICSTime(ev.EndRaw)
		if err1 != nil || err2 != nil {
			continue
		}

		windows = append(windows, holidayWindow{
			start: start,
			end:   end,
		})
	}

	return windows
}

func (s *CalendarService) shouldHideEvent(
	ev *ics.VEvent,
	cfg models.FilterConfig,
	windows []holidayWindow,
	hasHoliday bool,
) bool {
	rawSummary := ev.GetProperty("SUMMARY").Value
	baseKey := makeSeriesKey(rawSummary)
	uid := ev.GetProperty("UID").Value

	// -------------------------------------------------------
	// 🔥 FIX #1 — NEVER hide the holiday events themselves
	// -------------------------------------------------------
	for _, h := range cfg.HolidayUIDs {
		if uid == h {
			return false
		}
	}

	// Also protect all events with the SAME base name as a holiday
	// (so "Absent 01/01" is kept if ANY "Absent" was marked holiday)
	if hasHoliday {
		for _, w := range windows {
			start, _ := parseICSTime(ev.GetProperty("DTSTART").Value)
			end, _ := parseICSTime(ev.GetProperty("DTEND").Value)

			// If THIS event *is* one of the holiday windows → keep it
			if start.Equal(w.start) && end.Equal(w.end) {
				return false
			}
		}
	}

	// -------------------------------------------------------
	// 1) Explicit single-event hide (non-holiday only)
	// -------------------------------------------------------
	if s.isExplicitlyHidden(uid, cfg) {
		return true
	}

	// -------------------------------------------------------
	// 2) Hide whole fuzzy series (non-holiday only)
	// -------------------------------------------------------
	for hideKey := range cfg.HideSeries {
		if hideKey == baseKey {
			return true
		}
	}

	// -------------------------------------------------------
	// 3) Hide events that OVERLAP any holiday window
	// -------------------------------------------------------
	if hasHoliday {
		for _, w := range windows {
			if s.overlapsHoliday(ev, w) {
				return true
			}
		}
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
