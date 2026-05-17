package recipes_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
)

// ── iCal feed ─────────────────────────────────────────────────────────────────

func TestICalFeed_InvalidToken(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/ical/not-a-uuid.ics")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestICalFeed_UnknownToken(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		"/recipes/ical/00000000-0000-0000-0000-000000000099.ics")
	assert.Equal(t, http.StatusNotFound, tReq.Do(t).StatusCode)
}

func TestICalFeed_ValidToken(t *testing.T) {
	tReq := test.CreateRequestTester(getRoutes(), http.MethodPost, "/recipes/plans/new")
	tReq.SetContentType(test.FormContentType)
	tReq.SetFollowRedirect(false)
	tReq.SetData(dtos.CreatePlanDto{
		Name: "iCal Test Plan",
	})
	rs := tReq.Do(t)
	require.Equal(t, http.StatusSeeOther, rs.StatusCode)

	planPath := rs.Header.Get("Location")
	planID := strings.TrimPrefix(planPath, "/recipes/plans/")

	var icalToken string
	err := testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	icalReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	icalRS := icalReq.Do(t)
	assert.Equal(t, http.StatusOK, icalRS.StatusCode)
	assert.Equal(t, "text/calendar; charset=utf-8", icalRS.Header.Get("Content-Type"))
}

func TestICalFeed_WithMeals(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipe(t)
	addTestMeal(t, planID, recipeID)

	var icalToken string
	err := testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	rs := tReq.Do(t)
	require.Equal(t, http.StatusOK, rs.StatusCode)

	body, err := io.ReadAll(rs.Body)
	require.NoError(t, err)
	bodyStr := string(body)

	assert.Contains(t, bodyStr, "BEGIN:VEVENT")
	assert.Contains(t, bodyStr, "DTSTART:")
	assert.Contains(t, bodyStr, "DTEND:")
	assert.Contains(t, bodyStr, "DTSTAMP:")
	assert.Contains(t, bodyStr, "SUMMARY:Noon – Test Pasta")
}

// ── iCal feed slot / past filtering ──────────────────────────────────────────

func TestICalFeed_HidesSlot(t *testing.T) {
	planID := createTestPlan(t)
	recipeID := createTestRecipe(t)
	addTestMeal(t, planID, recipeID)

	_, err := testDB.Exec(
		t.Context(),
		`UPDATE recipes.plans SET ical_hide_slots = '{noon}' WHERE id = $1`,
		planID,
	)
	require.NoError(t, err)

	var icalToken string
	err = testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	rs := tReq.Do(t)
	require.Equal(t, http.StatusOK, rs.StatusCode)

	body, err := io.ReadAll(rs.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "BEGIN:VEVENT")
}

func TestICalFeed_HidesPastMeals(t *testing.T) {
	planID := createTestPlan(t)

	_, err := testDB.Exec(
		t.Context(),
		`UPDATE recipes.plans SET ical_hide_past = true WHERE id = $1`,
		planID,
	)
	require.NoError(t, err)

	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	_, err = testDB.Exec(
		t.Context(),
		`INSERT INTO recipes.plan_meals (plan_id, meal_date, meal_slot, custom_name, servings)
		 VALUES ($1, $2, 'noon', 'Yesterday meal', 2)`,
		planID,
		yesterday,
	)
	require.NoError(t, err)

	var icalToken string
	err = testDB.QueryRow(
		t.Context(),
		"SELECT ical_token::text FROM recipes.plans WHERE id = $1",
		planID,
	).Scan(&icalToken)
	require.NoError(t, err)

	tReq := test.CreateRequestTester(getRoutes(), http.MethodGet,
		fmt.Sprintf("/recipes/ical/%s.ics", icalToken))
	rs := tReq.Do(t)
	require.Equal(t, http.StatusOK, rs.StatusCode)

	body, err := io.ReadAll(rs.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "BEGIN:VEVENT")
}
