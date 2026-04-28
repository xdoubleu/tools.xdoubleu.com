package recipes

import (
	"fmt"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/recipes/internal/models"
)

// renderICalFeed produces an RFC 5545 VCALENDAR string for a meal plan.
// Events are all-day (DATE, not DATETIME) so they display correctly in Apple
// Calendar and Google Calendar without timezone concerns.
func renderICalFeed(plan *models.Plan, meals []models.PlanMeal) string {
	var sb strings.Builder

	writeln := func(s string) {
		sb.WriteString(s)
		sb.WriteString("\r\n")
	}

	writeln("BEGIN:VCALENDAR")
	writeln("VERSION:2.0")
	writeln("PRODID:-//tools.xdoubleu.com//Recipes//EN")
	writeln("CALSCALE:GREGORIAN")
	writeln("X-WR-CALNAME:" + escapeICalText(plan.Name))

	for _, meal := range meals {
		if meal.Recipe == nil {
			continue
		}
		dateStr := meal.MealDate.Format("20060102")
		slot := "Noon"
		if meal.MealSlot == "evening" {
			slot = "Evening"
		}
		summary := fmt.Sprintf("%s – %s (×%d)", slot, meal.Recipe.Name, meal.Servings)

		nextDay := meal.MealDate.AddDate(0, 0, 1).Format("20060102")
		dtstamp := time.Now().UTC().Format("20060102T150405Z")
		writeln("BEGIN:VEVENT")
		writeln("UID:" + meal.ID.String() + "@tools.xdoubleu.com")
		writeln("DTSTAMP:" + dtstamp)
		writeln("DTSTART;VALUE=DATE:" + dateStr)
		writeln("DTEND;VALUE=DATE:" + nextDay)
		writeln("SUMMARY:" + escapeICalText(summary))
		writeln("END:VEVENT")
	}

	writeln("END:VCALENDAR")
	return sb.String()
}

func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
