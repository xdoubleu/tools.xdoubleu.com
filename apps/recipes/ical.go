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

	hideSlots := make(map[string]bool, len(plan.ICalHideSlots))
	for _, s := range plan.ICalHideSlots {
		hideSlots[s] = true
	}
	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)

	writeln("BEGIN:VCALENDAR")
	writeln("VERSION:2.0")
	writeln("PRODID:-//tools.xdoubleu.com//Recipes//EN")
	writeln("CALSCALE:GREGORIAN")
	writeln("X-WR-CALNAME:" + escapeICalText(plan.Name))

	for _, meal := range meals {
		if hideSlots[meal.MealSlot] {
			continue
		}
		if plan.ICalHidePast && meal.MealDate.Before(today) {
			continue
		}
		dateStr := meal.MealDate.Format("20060102")

		var slot, dtstart, dtend string
		switch meal.MealSlot {
		case "breakfast":
			slot = "Breakfast"
			dtstart = dateStr + "T080000"
			dtend = dateStr + "T090000"
		case "evening":
			slot = "Evening"
			dtstart = dateStr + "T190000"
			dtend = dateStr + "T200000"
		default: // noon
			slot = "Noon"
			dtstart = dateStr + "T120000"
			dtend = dateStr + "T130000"
		}

		name := meal.CustomName
		if meal.Recipe != nil {
			name = meal.Recipe.Name
		}
		summary := fmt.Sprintf("%s – %s (×%d)", slot, name, meal.Servings)

		dtstamp := time.Now().UTC().Format("20060102T150405Z")
		writeln("BEGIN:VEVENT")
		writeln("UID:" + meal.ID.String() + "@tools.xdoubleu.com")
		writeln("DTSTAMP:" + dtstamp)
		writeln("DTSTART:" + dtstart)
		writeln("DTEND:" + dtend)
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
