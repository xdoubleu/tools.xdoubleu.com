package connect

import (
	"strconv"
	"time"

	"tools.xdoubleu.com/apps/books/internal/models"
)

func buildFinishedAt(existing *models.UserBook, newStatus string) []time.Time {
	if newStatus != models.StatusRead {
		return nil
	}
	if existing == nil {
		return []time.Time{time.Now()}
	}
	result := append([]time.Time{}, existing.FinishedAt...)
	if existing.Status != models.StatusRead {
		result = append(result, time.Now())
	}
	return result
}

func toggleTag(tags []string, tag string, enable bool) []string {
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != tag {
			result = append(result, t)
		}
	}
	if enable {
		result = append(result, tag)
	}
	return result
}

func parseRating(raw string) *int16 {
	if raw == "" || raw == "0" {
		return nil
	}
	n, err := strconv.ParseInt(raw, 10, 16)
	if err != nil || n <= 0 || n > 5 {
		return nil
	}
	r16 := int16(n)
	return &r16
}

func parseDateRangeFromStrings(dateStart, dateEnd string) (time.Time, time.Time) {
	end := time.Now()
	start := end.AddDate(-1, 0, 0)

	if dateStart != "" {
		if t, err := time.Parse(models.ProgressDateFormat, dateStart); err == nil {
			start = t
		}
	}
	if dateEnd != "" {
		if t, err := time.Parse(models.ProgressDateFormat, dateEnd); err == nil {
			end = t
		}
	}
	return start, end
}
