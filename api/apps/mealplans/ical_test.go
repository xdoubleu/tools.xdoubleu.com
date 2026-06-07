//nolint:testpackage // tests unexported helpers
package mealplans

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
)

// ── escapeICalText ────────────────────────────────────────────────────────────

func TestEscapeICalText_NoSpecialChars(t *testing.T) {
	assert.Equal(t, "plain text", escapeICalText("plain text"))
}

func TestEscapeICalText_Backslash(t *testing.T) {
	assert.Equal(t, `a\\b`, escapeICalText(`a\b`))
}

func TestEscapeICalText_Semicolon(t *testing.T) {
	assert.Equal(t, `a\;b`, escapeICalText("a;b"))
}

func TestEscapeICalText_Comma(t *testing.T) {
	assert.Equal(t, `a\,b`, escapeICalText("a,b"))
}

func TestEscapeICalText_Newline(t *testing.T) {
	assert.Equal(t, `line1\nline2`, escapeICalText("line1\nline2"))
}

func TestEscapeICalText_AllSpecial(t *testing.T) {
	out := escapeICalText("a\\b;c,d\ne")
	assert.Equal(t, `a\\b\;c\,d\ne`, out)
}

// ── renderICalFeed ────────────────────────────────────────────────────────────

func makeTestPlan(name string, hideSlots []string, hidePast bool) *models.Plan {
	return &models.Plan{ //nolint:exhaustruct // only relevant fields set in test
		ID:            uuid.New(),
		Name:          name,
		ICalHideSlots: hideSlots,
		ICalHidePast:  hidePast,
	}
}

func makeMeal(slot string, daysFromNow int) models.PlanMeal {
	date := time.Now().UTC().
		Truncate(24*time.Hour).
		AddDate(0, 0, daysFromNow)
	return models.PlanMeal{ //nolint:exhaustruct // only relevant fields set in test
		ID:         uuid.New(),
		MealSlot:   slot,
		MealDate:   date,
		CustomName: "Test Meal",
		Servings:   2,
	}
}

func TestRenderICalFeed_ContainsVCalendarWrapper(t *testing.T) {
	plan := makeTestPlan("My Meals", nil, false)
	out := renderICalFeed(plan, nil)
	assert.Contains(t, out, "BEGIN:VCALENDAR")
	assert.Contains(t, out, "END:VCALENDAR")
}

func TestRenderICalFeed_ContainsPlanName(t *testing.T) {
	plan := makeTestPlan("Dinner Plan", nil, false)
	out := renderICalFeed(plan, nil)
	assert.Contains(t, out, "X-WR-CALNAME:Dinner Plan")
}

func TestRenderICalFeed_NoonSlot(t *testing.T) {
	plan := makeTestPlan("P", nil, false)
	meals := []models.PlanMeal{makeMeal("noon", 1)}
	out := renderICalFeed(plan, meals)
	assert.Contains(t, out, "BEGIN:VEVENT")
	assert.Contains(t, out, "T120000")
	assert.Contains(t, out, "Noon")
}

func TestRenderICalFeed_BreakfastSlot(t *testing.T) {
	plan := makeTestPlan("P", nil, false)
	meals := []models.PlanMeal{makeMeal("breakfast", 1)}
	out := renderICalFeed(plan, meals)
	assert.Contains(t, out, "T080000")
	assert.Contains(t, out, "Breakfast")
}

func TestRenderICalFeed_EveningSlot(t *testing.T) {
	plan := makeTestPlan("P", nil, false)
	meals := []models.PlanMeal{makeMeal("evening", 1)}
	out := renderICalFeed(plan, meals)
	assert.Contains(t, out, "T190000")
	assert.Contains(t, out, "Evening")
}

func TestRenderICalFeed_HiddenSlotSkipped(t *testing.T) {
	plan := makeTestPlan("P", []string{"breakfast"}, false)
	meals := []models.PlanMeal{
		makeMeal("breakfast", 1),
		makeMeal("noon", 1),
	}
	out := renderICalFeed(plan, meals)
	lines := strings.Count(out, "BEGIN:VEVENT")
	assert.Equal(t, 1, lines)
}

func TestRenderICalFeed_HidePastSkipsPastMeals(t *testing.T) {
	plan := makeTestPlan("P", nil, true)
	meals := []models.PlanMeal{
		makeMeal("noon", -2), // past
		makeMeal("noon", 1),  // future
	}
	out := renderICalFeed(plan, meals)
	assert.Equal(t, 1, strings.Count(out, "BEGIN:VEVENT"))
}

func TestRenderICalFeed_RecipeNameTakesPriority(t *testing.T) {
	plan := makeTestPlan("P", nil, false)
	meal := makeMeal("noon", 1)
	meal.RecipeName = "Pasta Carbonara"
	out := renderICalFeed(plan, []models.PlanMeal{meal})
	assert.Contains(t, out, "Pasta Carbonara")
}

func TestRenderICalFeed_ExcludedEntryIncluded(t *testing.T) {
	plan := makeTestPlan("P", nil, false)
	entry := makeMeal("noon", 1)
	entry.CustomName = "Birthday Dinner"
	entry.ExcludeFromShoppingList = true
	meals := []models.PlanMeal{entry, makeMeal("evening", 1)}
	out := renderICalFeed(plan, meals)
	assert.Equal(t, 2, strings.Count(out, "BEGIN:VEVENT"))
	assert.Contains(t, out, "Birthday Dinner")
}

func TestRenderICalFeed_ExcludedEntryShowsServings(t *testing.T) {
	plan := makeTestPlan("P", nil, false)
	entry := makeMeal("noon", 1)
	entry.CustomName = "Birthday Dinner"
	entry.Servings = 4
	entry.ExcludeFromShoppingList = true
	out := renderICalFeed(plan, []models.PlanMeal{entry})
	assert.Contains(t, out, "Noon – Birthday Dinner (×4)")
}

func TestRenderICalFeed_UsesCRLF(t *testing.T) {
	plan := makeTestPlan("P", nil, false)
	out := renderICalFeed(plan, nil)
	assert.Contains(t, out, "\r\n")
}
