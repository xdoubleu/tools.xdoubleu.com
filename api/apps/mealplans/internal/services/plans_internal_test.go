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

// fakePlansStore implements plansStore in memory for family-scoping tests.
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
}

func (f *fakePlansStore) ListForFamily(
	_ context.Context, _ uuid.UUID, _, _ int32,
) ([]models.Plan, bool, error) {
	return nil, false, nil
}

func (f *fakePlansStore) GetByID(
	_ context.Context, _ uuid.UUID, familyID uuid.UUID,
) (*models.Plan, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.plan.FamilyID != familyID {
		return nil, &app.HTTPError{
			Status:  http.StatusNotFound,
			Message: "plan not found",
		}
	}
	cp := *f.plan
	return &cp, nil
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

func (f *fakePlansStore) Delete(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
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

// fakeFamilyStore maps a user to a fixed family ID for tests.
type fakeFamilyStore struct {
	families map[string]uuid.UUID
	err      error
}

func (f *fakeFamilyStore) EnsureFamily(
	_ context.Context, userID string,
) (uuid.UUID, error) {
	if f.err != nil {
		return uuid.Nil, f.err
	}
	return f.families[userID], nil
}

//nolint:gochecknoglobals //shared fixtures for family-scoping tests
var (
	familyA = uuid.New()
	familyB = uuid.New()
)

func newFamilyStore() *fakeFamilyStore {
	//nolint:exhaustruct //err intentionally zero
	return &fakeFamilyStore{
		families: map[string]uuid.UUID{
			"owner":  familyA,
			"member": familyA,
			"editor": familyA,
			"other":  familyB,
		},
	}
}

func newPlanFixture() *models.Plan {
	//nolint:exhaustruct //only fields relevant to family scoping
	return &models.Plan{ID: uuid.New(), OwnerUserID: "owner", FamilyID: familyA}
}

func planHTTPStatus(t *testing.T, err error) int {
	t.Helper()
	var httpErr *app.HTTPError
	require.ErrorAs(t, err, &httpErr)
	return httpErr.Status
}

func TestPlanUpdate_FamilyMemberAllowed(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture()}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	err := svc.Update(t.Context(), "editor", *store.plan)
	require.NoError(t, err)
	assert.True(t, store.updated)
}

func TestPlanUpdate_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture()}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	err := svc.Update(t.Context(), "other", *store.plan)
	assert.Equal(t, http.StatusNotFound, planHTTPStatus(t, err))
	assert.False(t, store.updated)
}

func TestPlanDelete_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture()}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	err := svc.Delete(t.Context(), uuid.New(), "other")
	assert.Equal(t, http.StatusNotFound, planHTTPStatus(t, err))
	assert.False(t, store.deleted)
}

func TestPlanMealMutations_OtherFamilyForbidden(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture()}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	err := svc.CreateMeal(
		t.Context(), store.plan.ID, "other",
		models.PlanMeal{}, //nolint:exhaustruct //empty meal is enough
	)
	assert.Equal(t, http.StatusNotFound, planHTTPStatus(t, err))
	assert.False(t, store.mealCreated)

	err = svc.DeleteMeal(t.Context(), uuid.New(), store.plan.ID, "other")
	assert.Equal(t, http.StatusNotFound, planHTTPStatus(t, err))
	assert.False(t, store.mealDeleted)

	err = svc.MoveMeal(
		t.Context(), uuid.New(), store.plan.ID, "other", time.Now(), "dinner",
	)
	assert.Equal(t, http.StatusNotFound, planHTTPStatus(t, err))
	assert.False(t, store.mealMoved)
}

func TestPlanMealMutations_AllowedForFamilyMember(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture()}
	svc := &PlanService{repo: store, family: newFamilyStore()}

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
	store := &fakePlansStore{plan: newPlanFixture()}
	svc := &PlanService{repo: store, family: newFamilyStore()}

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
	store := &fakePlansStore{plan: newPlanFixture(), getErr: getErr}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	_, _, _, err := svc.GetWithWeek(t.Context(), uuid.New(), "owner", 0)
	assert.ErrorIs(t, err, getErr)
}

func TestPlanGetWithWeek_PropagatesMealsError(t *testing.T) {
	mealsErr := errors.New("db error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture(), mealsErr: mealsErr}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	_, _, _, err := svc.GetWithWeek(t.Context(), store.plan.ID, "owner", 0)
	assert.ErrorIs(t, err, mealsErr)
}

func TestPlanCreate_ScopesToUsersFamily(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	//nolint:exhaustruct //other fields optional
	created, err := svc.Create(t.Context(), "member", models.Plan{Name: "Plan"})
	require.NoError(t, err)
	assert.Equal(t, "member", created.OwnerUserID)
	assert.Equal(t, familyA, created.FamilyID)
}

func TestPlanList_ScopesToUsersFamily(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{}
	svc := &PlanService{repo: store, family: newFamilyStore()}

	_, _, err := svc.List(t.Context(), "member", 10, 0)
	require.NoError(t, err)
}

// Every PlanService method resolves the caller's family before touching the
// repo; a family-resolution failure must propagate.
func TestFamilyResolutionErrors_Propagate(t *testing.T) {
	familyErr := errors.New("family error")
	//nolint:exhaustruct //unset fields are the fixture defaults
	family := &fakeFamilyStore{err: familyErr}
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakePlansStore{plan: newPlanFixture()}
	svc := &PlanService{repo: store, family: family}
	ctx := t.Context()

	_, _, err := svc.List(ctx, "member", 10, 0)
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.Get(ctx, store.plan.ID, "member")
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.GetMeals(ctx, store.plan.ID, "member", time.Now(), time.Now())
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.SuggestRecipes(ctx, store.plan.ID, "member", time.Now(), "noon")
	assert.ErrorIs(t, err, familyErr)

	//nolint:exhaustruct //other fields optional
	_, err = svc.Create(ctx, "member", models.Plan{Name: "Plan"})
	assert.ErrorIs(t, err, familyErr)

	err = svc.Delete(ctx, store.plan.ID, "member")
	assert.ErrorIs(t, err, familyErr)
}
