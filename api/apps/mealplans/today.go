package mealplans

import (
	"context"
	"time"
)

// TodayMeal is one planned meal for the Today page.
type TodayMeal struct {
	Date string // yyyy-MM-dd
	Slot string // breakfast, noon or evening
	Name string
}

const (
	dateLayout        = "2006-01-02"
	slotEvening       = "evening"
	todayPlansLimit   = 20
	todayWindowInDays = 2
)

// ListTodayMeals returns the meals planned for today and tomorrow's evening
// meal across the user's family plans, ordered by date and slot.
func (a *MealPlans) ListTodayMeals(
	ctx context.Context,
	userID string,
	today time.Time,
) ([]TodayMeal, error) {
	plans, _, err := a.services.Plans.List(ctx, userID, todayPlansLimit, 0)
	if err != nil {
		return nil, err
	}

	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(todayWindowInDays * hoursPerDay * time.Hour)
	todayKey := start.Format(dateLayout)
	tomorrowKey := start.AddDate(0, 0, 1).Format(dateLayout)

	meals := []TodayMeal{}
	for _, plan := range plans {
		planMeals, mealsErr := a.services.Plans.GetMeals(ctx, plan.ID, userID, start, end)
		if mealsErr != nil {
			return nil, mealsErr
		}
		for _, m := range planMeals {
			key := m.MealDate.Format(dateLayout)
			if key != todayKey && (key != tomorrowKey || m.MealSlot != slotEvening) {
				continue
			}
			name := m.CustomName
			if name == "" {
				name = m.RecipeName
			}
			meals = append(meals, TodayMeal{Date: key, Slot: m.MealSlot, Name: name})
		}
	}
	return meals, nil
}
