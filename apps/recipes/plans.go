package recipes

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/services"
)

type planDay struct {
	Date    time.Time
	Noon    *models.PlanMeal
	Evening *models.PlanMeal
}

// ── List plans ────────────────────────────────────────────────────────────────

func (a *Recipes) listPlansHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	planList, err := a.services.Plans.List(r.Context(), user.ID)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "plans_list.html", map[string]any{
		"Plans": planList,
	})
	return nil
}

// ── New plan form ─────────────────────────────────────────────────────────────

func (a *Recipes) newPlanFormHandler(w http.ResponseWriter, _ *http.Request) error {
	tpltools.RenderWithPanic(a.Tpl, w, "plans_form.html", map[string]any{
		//nolint:exhaustruct // other fields optional
		"Plan":   models.Plan{},
		"Action": "/recipes/plans/new",
	})
	return nil
}

// ── Create plan ───────────────────────────────────────────────────────────────

func (a *Recipes) createPlanHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)

	var dto dtos.CreatePlanDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	plan, err := parsePlanDto(dto)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid dates: " + err.Error(),
		}
	}

	created, err := a.services.Plans.Create(r.Context(), user.ID, plan)
	if err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans/"+created.ID.String(), http.StatusSeeOther)
	return nil
}

// ── View plan ─────────────────────────────────────────────────────────────────

func (a *Recipes) viewPlanHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	plan, err := a.services.Plans.Get(r.Context(), id, user.ID)
	if err != nil {
		return err
	}

	days := buildCalendarDays(plan)

	recipeList, err := a.services.Recipes.List(r.Context(), user.ID)
	if err != nil {
		return err
	}

	icalURL := fmt.Sprintf("/recipes/ical/%s.ics", plan.ICalToken)

	tpltools.RenderWithPanic(a.Tpl, w, "plans_view.html", map[string]any{
		"Plan":    plan,
		"Days":    days,
		"Recipes": recipeList,
		"ICalURL": icalURL,
		"IsOwner": plan.OwnerUserID == user.ID,
	})
	return nil
}

// ── Edit plan form ────────────────────────────────────────────────────────────

func (a *Recipes) editPlanFormHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	plan, err := a.services.Plans.Get(r.Context(), id, user.ID)
	if err != nil {
		return err
	}

	tpltools.RenderWithPanic(a.Tpl, w, "plans_form.html", map[string]any{
		"Plan":   plan,
		"Action": "/recipes/plans/" + id.String() + "/edit",
	})
	return nil
}

// ── Update plan ───────────────────────────────────────────────────────────────

func (a *Recipes) updatePlanHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	var dto dtos.CreatePlanDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	plan, err := parsePlanDto(dto)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid dates: " + err.Error(),
		}
	}
	plan.ID = id

	if err = a.services.Plans.Update(r.Context(), user.ID, plan); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans/"+id.String(), http.StatusSeeOther)
	return nil
}

// ── Delete plan ───────────────────────────────────────────────────────────────

func (a *Recipes) deletePlanHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	if err = a.services.Plans.Delete(r.Context(), id, user.ID); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans", http.StatusSeeOther)
	return nil
}

// ── Add meal ──────────────────────────────────────────────────────────────────

func (a *Recipes) addMealHandler(w http.ResponseWriter, r *http.Request) error {
	planID, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	var dto dtos.AddMealDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	mealDate, err := time.Parse("2006-01-02", dto.MealDate)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid date",
		}
	}

	recipeID, err := uuid.Parse(dto.RecipeID)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid recipe",
		}
	}

	servings := dto.Servings
	if servings <= 0 {
		servings = 2
	}

	//nolint:exhaustruct //other fields optional
	meal := models.PlanMeal{
		MealDate: mealDate,
		MealSlot: dto.MealSlot,
		RecipeID: recipeID,
		Servings: servings,
	}

	if err = a.services.Plans.AddMeal(r.Context(), planID, user.ID, meal); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans/"+planID.String(), http.StatusSeeOther)
	return nil
}

// ── Delete meal ───────────────────────────────────────────────────────────────

func (a *Recipes) deleteMealHandler(w http.ResponseWriter, r *http.Request) error {
	planID, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	mealID, err := uuid.Parse(r.PathValue("mealId"))
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Meal not found",
		}
	}

	if err = a.services.Plans.DeleteMeal(
		r.Context(), mealID, planID, user.ID,
	); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans/"+planID.String(), http.StatusSeeOther)
	return nil
}

// ── Share plan ────────────────────────────────────────────────────────────────

func (a *Recipes) sharePlanHandler(w http.ResponseWriter, r *http.Request) error {
	planID, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	var dto dtos.SharePlanDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	if err = a.services.Plans.ShareByEmail(
		r.Context(), planID, user.ID, dto.Email, dto.CanEdit,
	); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans/"+planID.String(), http.StatusSeeOther)
	return nil
}

// ── iCal feed (public, no auth) ───────────────────────────────────────────────

func (a *Recipes) icalFeedHandler(w http.ResponseWriter, r *http.Request) {
	// Extract token from path: /recipes/plans/ical/<token>.ics
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

// ── Helpers ───────────────────────────────────────────────────────────────────

func parsePlanUUID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}

func parsePlanDto(dto dtos.CreatePlanDto) (models.Plan, error) {
	startDate, err := time.Parse("2006-01-02", dto.StartDate)
	if err != nil {
		return models.Plan{}, err
	}
	endDate, err := time.Parse("2006-01-02", dto.EndDate)
	if err != nil {
		return models.Plan{}, err
	}
	//nolint:exhaustruct //other fields optional
	return models.Plan{
		Name:      dto.Name,
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

func buildCalendarDays(plan *models.Plan) []planDay {
	mealsByDateSlot := make(map[string]*models.PlanMeal)
	for i := range plan.Meals {
		meal := &plan.Meals[i]
		key := meal.MealDate.Format("2006-01-02") + ":" + meal.MealSlot
		mealsByDateSlot[key] = meal
	}

	var days []planDay
	for d := plan.StartDate; !d.After(plan.EndDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		//nolint:exhaustruct //other fields optional
		day := planDay{Date: d}
		if m, ok := mealsByDateSlot[dateStr+":noon"]; ok {
			day.Noon = m
		}
		if m, ok := mealsByDateSlot[dateStr+":evening"]; ok {
			day.Evening = m
		}
		days = append(days, day)
	}
	return days
}
