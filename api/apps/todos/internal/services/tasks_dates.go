package services

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/internal/app"
)

const (
	ordinalSecond  = 2
	ordinalThird   = 3
	ordinalFourth  = 4
	ordinalFifth   = 5
	daysInWeek     = 7
	twoRuleParts   = 2
	threeRuleParts = 3
)

// weekdayPattern matches "next <weekday>" with an optional "at HH" or "at HH:MM".
var weekdayPattern = regexp.MustCompile(
	`(?i)\bnext\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)` +
		`(?:\s+at\s+(\d{1,2})(?::(\d{2}))?)?\b`,
)

var recurringInTitlePattern = regexp.MustCompile(
	`(?i)\bevery\s+((first|second|third|fourth|fifth|last)\s+)?` +
		`(monday|tuesday|wednesday|thursday|friday|saturday|sunday|\d+\s+days?)\b`,
)

var everyPattern = regexp.MustCompile(
	`(?i)^\s*every\s+((first|second|third|fourth|fifth|last)\s+)?([a-z]+|\d+\s+days?)\s*$`,
)

var isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type recurringParseResult struct {
	recurDays int
	recurRule string
}

func parseScheduleDTO(
	dto dtos.SaveTaskDto,
	now time.Time,
) (*time.Time, *time.Time, int, string, error) {
	due, dueRecurring, err := parseHumanDate(dto.DueDate, now, true)
	if err != nil {
		return nil, nil, 0, "", &app.HTTPError{
			Status: http.StatusBadRequest,
			Message: "Invalid due date. Use e.g. today, tomorrow, next thursday," +
				" every thursday, or every first sunday.",
		}
	}
	deadline, _, err := parseHumanDate(dto.Deadline, now, false)
	if err != nil {
		return nil, nil, 0, "", &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid deadline. Use e.g. today, tomorrow, next thursday, or YYYY-MM-DD.",
		}
	}

	recurDays := dto.RecurDays
	recurRule := ""
	if strings.TrimSpace(dto.Recur) != "" {
		recurDays, recurRule, err = parseRecurOnly(dto.Recur, now)
		if err != nil {
			return nil, nil, 0, "", &app.HTTPError{
				Status: http.StatusBadRequest,
				Message: "Invalid recur value." +
					" Use e.g. every thursday, every first sunday, or every 7 days.",
			}
		}
	}
	if dueRecurring.recurDays > 0 && strings.TrimSpace(dto.Recur) == "" {
		recurDays = dueRecurring.recurDays
	}
	if dueRecurring.recurRule != "" && strings.TrimSpace(dto.Recur) == "" {
		recurRule = dueRecurring.recurRule
	}
	if recurRule == "" && recurDays > 0 {
		recurRule = "days:" + strconv.Itoa(recurDays)
	}

	return due, deadline, recurDays, recurRule, nil
}

func parseHumanDate(
	input string,
	now time.Time,
	allowRecurring bool,
) (*time.Time, recurringParseResult, error) {
	empty := recurringParseResult{recurDays: 0, recurRule: ""}
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, empty, nil
	}

	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
		return &d, empty, nil
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case "today":
		d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &d, empty, nil
	case "tomorrow":
		d := now.AddDate(0, 0, 1)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return &t, empty, nil
	}

	if m := everyPattern.FindStringSubmatch(lower); m != nil {
		if !allowRecurring {
			return nil, empty, strconv.ErrSyntax
		}
		return parseEveryDate(m, now)
	}

	if wd, ok := weekdayByName(lower); ok {
		d := nextWeekday(now, wd)
		return &d, empty, nil
	}

	if m := weekdayPattern.FindStringSubmatch(lower); m != nil {
		wd, ok := weekdayByName(m[1])
		if !ok {
			return nil, empty, strconv.ErrSyntax
		}
		d := nextWeekday(now, wd)
		return &d, empty, nil
	}

	return nil, empty, strconv.ErrSyntax
}

func parseEveryDate(
	m []string,
	now time.Time,
) (*time.Time, recurringParseResult, error) {
	empty := recurringParseResult{recurDays: 0, recurRule: ""}
	ordinalWord := strings.ToLower(strings.TrimSpace(m[2]))
	everyBody := strings.ToLower(strings.TrimSpace(m[3]))
	wd, wdOK := weekdayByName(everyBody)
	if wdOK && ordinalWord == "" {
		d := nextWeekday(now, wd)
		return &d, recurringParseResult{
			recurDays: daysInWeek,
			recurRule: "weekday:" + strconv.Itoa(int(wd)),
		}, nil
	}
	if wdOK && ordinalWord != "" {
		ordinal, ordOK := ordinalByName(ordinalWord)
		if !ordOK {
			return nil, empty, strconv.ErrSyntax
		}
		d, monthlyOK := nextMonthlyWeekday(now, wd, ordinal)
		if !monthlyOK {
			return nil, empty, strconv.ErrSyntax
		}
		return &d, recurringParseResult{
			recurDays: 0,
			recurRule: "monthweekday:" + strconv.Itoa(ordinal) +
				":" + strconv.Itoa(int(wd)),
		}, nil
	}
	parts := strings.Fields(everyBody)
	if len(parts) == twoRuleParts && strings.HasPrefix(parts[1], "day") {
		if n, ok := parsePositiveInt(parts[0]); ok {
			d := now.AddDate(0, 0, n)
			t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
			return &t, recurringParseResult{
				recurDays: n,
				recurRule: "days:" + strconv.Itoa(n),
			}, nil
		}
	}
	return nil, empty, strconv.ErrSyntax
}

func parseRecurOnly(input string, now time.Time) (int, string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, "", nil
	}
	if n, ok := parsePositiveInt(trimmed); ok {
		return n, "days:" + strconv.Itoa(n), nil
	}
	_, recurring, err := parseHumanDate(trimmed, now, true)
	if err != nil {
		return 0, "", err
	}
	if recurring.recurRule == "" {
		return 0, "", strconv.ErrSyntax
	}
	return recurring.recurDays, recurring.recurRule, nil
}

func parsePositiveInt(s string) (int, bool) {
	n64, parseErr := strconv.ParseInt(s, 10, 32)
	if parseErr != nil || n64 <= 0 {
		return 0, false
	}
	return int(n64), true
}

func parseDatePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

// weekdayPattern matches "next <weekday>" with an optional "at HH" or "at HH:MM".

func weekdayByName(name string) (time.Weekday, bool) {
	m := map[string]time.Weekday{
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
		"sunday":    time.Sunday,
	}
	wd, ok := m[strings.ToLower(name)]
	return wd, ok
}

// parseDateFromTitle extracts a natural-language due date phrase from title and
// returns cleaned title, due date, and optional recurrence metadata.
func parseDateFromTitle(title string, now time.Time) (string, *time.Time, string, int) {
	lower := strings.ToLower(title)

	if loc := recurringInTitlePattern.FindStringIndex(lower); loc != nil {
		phrase := strings.TrimSpace(title[loc[0]:loc[1]])
		if d, recurring, err := parseHumanDate(phrase, now, true); err == nil {
			cleaned := strings.TrimSpace(title[:loc[0]] + title[loc[1]:])
			return cleaned, d, recurring.recurRule, recurring.recurDays
		}
	}

	if loc := weekdayPattern.FindStringIndex(lower); loc != nil {
		m := weekdayPattern.FindStringSubmatch(lower)
		wd, _ := weekdayByName(m[1])
		d := nextWeekday(now, wd)
		cleaned := strings.TrimSpace(title[:loc[0]] + title[loc[1]:])
		return cleaned, &d, "", 0
	}

	if idx := strings.Index(lower, "tomorrow"); idx >= 0 {
		d := now.AddDate(0, 0, 1)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		cleaned := strings.TrimSpace(title[:idx] + title[idx+len("tomorrow"):])
		return cleaned, &t, "", 0
	}

	if idx := strings.Index(lower, "today"); idx >= 0 {
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		cleaned := strings.TrimSpace(title[:idx] + title[idx+len("today"):])
		return cleaned, &t, "", 0
	}

	return title, nil, "", 0
}

func nextWeekday(from time.Time, wd time.Weekday) time.Time {
	days := int(wd) - int(from.Weekday())
	if days <= 0 {
		days += 7
	}
	d := from.AddDate(0, 0, days)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, from.Location())
}

func ordinalByName(name string) (int, bool) {
	m := map[string]int{
		"first":  1,
		"second": ordinalSecond,
		"third":  ordinalThird,
		"fourth": ordinalFourth,
		"fifth":  ordinalFifth,
		"last":   -1,
	}
	v, ok := m[strings.ToLower(name)]
	return v, ok
}

func ordinalToName(o int) string {
	switch o {
	case 1:
		return "first"
	case ordinalSecond:
		return "second"
	case ordinalThird:
		return "third"
	case ordinalFourth:
		return "fourth"
	case ordinalFifth:
		return "fifth"
	case -1:
		return "last"
	default:
		return ""
	}
}

func nthWeekdayOfMonth(
	year int,
	month time.Month,
	wd time.Weekday,
	ordinal int,
	loc *time.Location,
) (time.Time, bool) {
	if ordinal == 0 || ordinal < -1 || ordinal > 5 {
		return time.Time{}, false
	}
	if ordinal == -1 {
		last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc)
		diff := (int(last.Weekday()) - int(wd) + daysInWeek) % daysInWeek
		d := last.AddDate(0, 0, -diff)
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc), true
	}
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	diff := (int(wd) - int(first.Weekday()) + daysInWeek) % daysInWeek
	day := 1 + diff + (ordinal-1)*daysInWeek
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if day > lastDay {
		return time.Time{}, false
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc), true
}

func nextMonthlyWeekday(
	from time.Time,
	wd time.Weekday,
	ordinal int,
) (time.Time, bool) {
	base := time.Date(
		from.Year(),
		from.Month(),
		from.Day(),
		0,
		0,
		0,
		0,
		from.Location(),
	)
	for offset := 0; offset < 24; offset++ {
		m := from.AddDate(0, offset, 0)
		candidate, ok := nthWeekdayOfMonth(
			m.Year(), m.Month(), wd, ordinal, from.Location(),
		)
		if !ok {
			continue
		}
		if candidate.After(base) {
			return candidate, true
		}
	}
	return time.Time{}, false
}

func nextRecurringDue(now time.Time, rule string, fallbackDays int) (*time.Time, int) {
	if rule == "" {
		if fallbackDays <= 0 {
			return nil, 0
		}
		d := now.AddDate(0, 0, fallbackDays)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return &t, fallbackDays
	}
	parts := strings.Split(rule, ":")
	switch parts[0] {
	case "days":
		if len(parts) != twoRuleParts {
			return nil, fallbackDays
		}
		n, ok := parsePositiveInt(parts[1])
		if !ok {
			return nil, fallbackDays
		}
		d := now.AddDate(0, 0, n)
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
		return &t, n
	case "weekday":
		if len(parts) != twoRuleParts {
			return nil, fallbackDays
		}
		w, err := strconv.Atoi(parts[1])
		if err != nil || w < 0 || w > 6 {
			return nil, fallbackDays
		}
		t := nextWeekday(now, time.Weekday(w))
		return &t, daysInWeek
	case "monthweekday":
		if len(parts) != threeRuleParts {
			return nil, fallbackDays
		}
		ordinal, err1 := strconv.Atoi(parts[1])
		w, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || w < 0 || w > 6 {
			return nil, fallbackDays
		}
		t, ok := nextMonthlyWeekday(now, time.Weekday(w), ordinal)
		if !ok {
			return nil, fallbackDays
		}
		return &t, 0
	default:
		return nil, fallbackDays
	}
}

func recurRuleToInput(rule string) string {
	parts := strings.Split(rule, ":")
	switch parts[0] {
	case "days":
		if len(parts) == twoRuleParts {
			return "every " + parts[1] + " days"
		}
	case "weekday":
		if len(parts) == twoRuleParts {
			if w, err := strconv.Atoi(parts[1]); err == nil && w >= 0 && w <= 6 {
				return "every " + strings.ToLower(time.Weekday(w).String())
			}
		}
	case "monthweekday":
		if len(parts) == threeRuleParts {
			o, err1 := strconv.Atoi(parts[1])
			w, err2 := strconv.Atoi(parts[2])
			if err1 == nil && err2 == nil && w >= 0 && w <= 6 {
				return "every " + ordinalToName(o) + " " +
					strings.ToLower(time.Weekday(w).String())
			}
		}
	}
	return ""
}
