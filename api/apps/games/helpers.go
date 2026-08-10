package games

import (
	"time"

	"tools.xdoubleu.com/apps/games/internal/models"
)

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
