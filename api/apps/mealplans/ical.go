package mealplans

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
)

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
	writeln("PRODID:-//tools.xdoubleu.com//MealPlans//EN")
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
		case models.SlotBreakfast:
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

		// Events are planning-only entries with no recipe or meaningful
		// serving count, so they show just their name.
		var summary string
		if meal.IsEvent {
			summary = fmt.Sprintf("%s – %s", slot, meal.CustomName)
		} else {
			name := displayCustomName(meal.CustomName)
			if meal.RecipeName != "" {
				name = meal.RecipeName
			}
			summary = fmt.Sprintf("%s – %s (×%d)", slot, name, meal.Servings)
		}

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

func (a *MealPlans) icalFeedHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	raw := parts[len(parts)-1]
	raw = strings.TrimSuffix(raw, ".ics")

	token, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}

	plan, err := a.services.Plans.GetByICalToken(r.Context(), token)
	if err != nil {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="meal-plan.ics"`)
	if _, err = fmt.Fprint(w, renderICalFeed(plan, plan.Meals)); err != nil {
		a.Logger.ErrorContext(
			r.Context(),
			"failed to write ical response",
			"error",
			err,
		)
	}
}
