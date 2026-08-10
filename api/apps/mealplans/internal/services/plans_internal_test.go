package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	"tools.xdoubleu.com/internal/app"
)

// fakePlansStore implements plansStore in memory for permission tests.
type fakePlansStore struct {
	plan          *models.Plan
	getErr        error
	mealsInWindow []models.PlanMeal
	mealsErr      error

	updated     bool
	deleted     bool
	mealCreated bool
	mealDeleted bool
	mealMoved   bool
	shared      bool
	unshared    bool
}

func (f *fakePlansStore) ListForUser(
	_ context.Context, _ string, _, _ int32,
) ([]models.Plan, bool, error) {
	return nil, false, nil
}

func (f *fakePlansStore) GetByID(
	_ context.Context, _ uuid.UUID, _ string,
) (*models.Plan, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	cp := *f.plan
	return &cp, nil
}

func (f *fakePlansStore) GetSharedWith(
	_ context.Context, _ uuid.UUID, _ string,
) ([]models.PlanSharedUser, error) {
	return nil, nil
}

func (f *fakePlansStore) GetMealsInWindow(
	_ context.Context, _ uuid.UUID, _, _ time.Time,
) ([]models.PlanMeal, error) {
	if f.mealsErr != nil {
		return nil, f.mealsErr
	}
	return f.mealsInWindow, nil
}

func (f *fakePlansStore) SuggestRecipes(
	_ context.Context, _ uuid.UUID, _ time.Time, _ string, _ int,
) ([]models.RecipeSuggestion, error) {
	return nil, nil
}

func (f *fakePlansStore) GetByICalToken(
	_ context.Context, _ uuid.UUID,
) (*models.Plan, error) {
	cp := *f.plan
	return &cp, nil
}

func (f *fakePlansStore) Create(
	_ context.Context, plan models.Plan,
) (*models.Plan, error) {
	return &plan, nil
}

func (f *fakePlansStore) Update(_ context.Context, _ models.Plan) error {
	f.updated = true
	return nil
}

func (f *fakePlansStore) Delete(_ context.Context, _ uuid.UUID, _ string) error {
	f.deleted = true
	return nil
}

func (f *fakePlansStore) CreateMeal(
	_ context.Context, meal models.PlanMeal,
) (*models.PlanMeal, error) {
	f.mealCreated = true
	return &meal, nil
}

func (f *fakePlansStore) DeleteMeal(_ context.Context, _, _ uuid.UUID) error {
	f.mealDeleted = true
	return nil
}

func (f *fakePlansStore) MoveMeal(
	_ context.Context, _, _ uuid.UUID, _ time.Time, _ string,
) error {
	f.mealMoved = true
	return nil
}

func (f *fakePlansStore) UnshareUser(
	_ context.Context, _ uuid.UUID, _ string,
) error {
	f.unshared = true
	return nil
}

func (f *fakePlansStore) SharePlan(
	_ context.Context, _ uuid.UUID, _ string, _ bool,
) error {
	f.shared = true
	return nil
}

func newPlanFixture(canEdit bool) *models.Plan {
	//nolint:exhaustruct //only fields relevant to permissions
	return &models.Plan{ID: uuid.New(), OwnerUserID: "owner", CanEdit: canEdit}
}

func planHTTPStatus(t *testing.T, err error) int {
	t.Helper()
	var httpErr *app.HTTPError
	require.ErrorAs(t, err, &httpErr)
	return httpErr.Status
}

func TestPlanUpdate_NonOwnerForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(true)}
	svc := &PlanService{repo: store}

	err := svc.Update(t.Context(), "editor", *store.plan)
	assert.Equal(t, http.StatusForbidden, planHTTPStatus(t, err))
	assert.False(t, store.updated)
}

func TestPlanDelete_NonOwnerForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(true)}
	svc := &PlanService{repo: store}

	err := svc.Delete(t.Context(), uuid.New(), "editor")
	assert.Equal(t, http.StatusForbidden, planHTTPStatus(t, err))
	assert.False(t, store.deleted)
}

func TestPlanMealMutations_RequireEditAccess(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(false)}
	svc := &PlanService{repo: store}
	viewer := "view-only-user"

	err := svc.CreateMeal(
		t.Context(), store.plan.ID, viewer,
		models.PlanMeal{}, //nolint:exhaustruct //empty meal is enough
	)
	assert.Equal(t, http.StatusForbidden, planHTTPStatus(t, err))
	assert.False(t, store.mealCreated)

	err = svc.DeleteMeal(t.Context(), uuid.New(), store.plan.ID, viewer)
	assert.Equal(t, http.StatusForbidden, planHTTPStatus(t, err))
	assert.False(t, store.mealDeleted)

	err = svc.MoveMeal(
		t.Context(), uuid.New(), store.plan.ID, viewer, time.Now(), "dinner",
	)
	assert.Equal(t, http.StatusForbidden, planHTTPStatus(t, err))
	assert.False(t, store.mealMoved)
}

func TestPlanMealMutations_AllowedWithEditAccess(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(true)}
	svc := &PlanService{repo: store}

	require.NoError(t, svc.CreateMeal(
		t.Context(), store.plan.ID, "editor",
		models.PlanMeal{}, //nolint:exhaustruct //empty meal is enough
	))
	require.NoError(t, svc.DeleteMeal(t.Context(), uuid.New(), store.plan.ID, "editor"))
	require.NoError(t, svc.MoveMeal(
		t.Context(), uuid.New(), store.plan.ID, "editor", time.Now(), "lunch",
	))
	assert.True(t, store.mealCreated)
	assert.True(t, store.mealDeleted)
	assert.True(t, store.mealMoved)
}

func TestWeekWindow_OffsetZeroIsCurrentWeek(t *testing.T) {
	start, end := WeekWindow(0)
	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)

	assert.Equal(t, today, start)
	assert.Equal(t, 6*24*time.Hour, end.Sub(start))
}

func TestWeekWindow_OffsetShiftsByWeeks(t *testing.T) {
	start1, _ := WeekWindow(1)
	start0, _ := WeekWindow(0)
	assert.Equal(t, 7*24*time.Hour, start1.Sub(start0))

	startNeg1, _ := WeekWindow(-1)
	assert.Equal(t, -7*24*time.Hour, startNeg1.Sub(start0))
}

func TestPlanGetWithWeek_ComputesWindowAndMeals(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(true)}
	svc := &PlanService{repo: store}

	plan, windowStart, windowEnd, err := svc.GetWithWeek(
		t.Context(), store.plan.ID, "owner", 2,
	)
	require.NoError(t, err)
	require.NotNil(t, plan)

	wantStart, wantEnd := WeekWindow(2)
	assert.Equal(t, wantStart, windowStart)
	assert.Equal(t, wantEnd, windowEnd)
}

func TestPlanGetWithWeek_PropagatesGetError(t *testing.T) {
	getErr := errors.New("db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(true), getErr: getErr}
	svc := &PlanService{repo: store}

	_, _, _, err := svc.GetWithWeek(t.Context(), uuid.New(), "owner", 0)
	assert.ErrorIs(t, err, getErr)
}

func TestPlanGetWithWeek_PropagatesMealsError(t *testing.T) {
	mealsErr := errors.New("db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(true), mealsErr: mealsErr}
	svc := &PlanService{repo: store}

	_, _, _, err := svc.GetWithWeek(t.Context(), store.plan.ID, "owner", 0)
	assert.ErrorIs(t, err, mealsErr)
}

func TestPlanSharing_OwnerOnly(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(true)}
	svc := &PlanService{repo: store}

	err := svc.Share(t.Context(), store.plan.ID, "editor", "friend", false)
	assert.Equal(t, http.StatusForbidden, planHTTPStatus(t, err))
	assert.False(t, store.shared)

	err = svc.Unshare(t.Context(), store.plan.ID, "editor", "friend")
	assert.Equal(t, http.StatusForbidden, planHTTPStatus(t, err))
	assert.False(t, store.unshared)

	require.NoError(t, svc.Share(t.Context(), store.plan.ID, "owner", "friend", true))
	require.NoError(t, svc.Unshare(t.Context(), store.plan.ID, "owner", "friend"))
	assert.True(t, store.shared)
	assert.True(t, store.unshared)
}
