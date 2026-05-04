package recipes

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	tpltools "github.com/xdoubleu/essentia/v4/pkg/tpl"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/services"
)

const (
	daysPerWeek = 7
	hoursPerDay = 24
)

type planDay struct {
	Date      time.Time
	Breakfast *models.PlanMeal
	Noon      *models.PlanMeal
	Evening   *models.PlanMeal
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
		"Plan":          models.Plan{},
		"Action":        "/recipes/plans/new",
		"HideBreakfast": false,
		"HideNoon":      false,
		"HideEvening":   false,
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

	//nolint:exhaustruct // other fields set by service
	plan := models.Plan{Name: dto.Name}

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

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, parseErr := strconv.Atoi(v); parseErr == nil {
			offset = n
		}
	}

	windowStart := time.Now().UTC().Truncate(hoursPerDay*time.Hour).
		AddDate(0, 0, offset*daysPerWeek)
	windowEnd := windowStart.AddDate(0, 0, daysPerWeek-1)

	meals, err := a.services.Plans.GetMeals(
		r.Context(),
		id,
		user.ID,
		windowStart,
		windowEnd,
	)
	if err != nil {
		return err
	}
	plan.Meals = meals

	days := buildCalendarDays(windowStart, windowEnd, meals)

	recipeList, err := a.services.Recipes.List(r.Context(), user.ID)
	if err != nil {
		return err
	}

	contactList, err := a.contacts.List(r.Context(), user.ID)
	if err != nil {
		return err
	}

	icalURL := fmt.Sprintf("/recipes/ical/%s.ics", plan.ICalToken)

	tpltools.RenderWithPanic(a.Tpl, w, "plans_view.html", map[string]any{
		"Plan":        plan,
		"Days":        days,
		"Recipes":     recipeList,
		"Contacts":    contactList,
		"ICalURL":     icalURL,
		"IsOwner":     plan.OwnerUserID == user.ID,
		"Offset":      offset,
		"PrevOffset":  offset - 1,
		"NextOffset":  offset + 1,
		"WindowStart": windowStart,
		"WindowEnd":   windowEnd,
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
		"Plan":          plan,
		"Action":        "/recipes/plans/" + id.String() + "/edit",
		"HideBreakfast": slices.Contains(plan.ICalHideSlots, "breakfast"),
		"HideNoon":      slices.Contains(plan.ICalHideSlots, "noon"),
		"HideEvening":   slices.Contains(plan.ICalHideSlots, "evening"),
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

	var dto dtos.UpdatePlanDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	hideSlots := dto.ICalHideSlots
	if hideSlots == nil {
		hideSlots = []string{}
	}

	//nolint:exhaustruct // other fields set by service
	plan := models.Plan{
		ID:            id,
		Name:          dto.Name,
		ICalHideSlots: hideSlots,
		ICalHidePast:  dto.ICalHidePast,
	}

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

	var recipeID *uuid.UUID
	if dto.RecipeID != "" {
		parsed, parseErr := uuid.Parse(dto.RecipeID)
		if parseErr != nil {
			return &services.HTTPError{
				Status:  http.StatusBadRequest,
				Message: "Invalid recipe",
			}
		}
		recipeID = &parsed
	}

	customName := strings.TrimSpace(dto.CustomName)
	if recipeID == nil && customName == "" {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "A recipe or meal name is required",
		}
	}

	servings := dto.Servings
	if servings <= 0 {
		servings = 2
	}

	//nolint:exhaustruct //other fields optional
	meal := models.PlanMeal{
		MealDate:   mealDate,
		MealSlot:   dto.MealSlot,
		RecipeID:   recipeID,
		CustomName: customName,
		Servings:   servings,
	}

	if err = a.services.Plans.AddMeal(r.Context(), planID, user.ID, meal); err != nil {
		return err
	}

	// Preserve the offset so the user returns to the same week view.
	redirect := "/recipes/plans/" + planID.String()
	//nolint:gosec // form body already size-limited by httptools.ReadForm above
	if offset := r.FormValue("offset"); offset != "" {
		redirect += "?offset=" + offset
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
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

	redirect := "/recipes/plans/" + planID.String()
	if offset := r.URL.Query().Get("offset"); offset != "" {
		redirect += "?offset=" + offset
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
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

	if err = a.services.Plans.Share(
		r.Context(), planID, user.ID, dto.ContactUserID, dto.CanEdit,
	); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans/"+planID.String(), http.StatusSeeOther)
	return nil
}

// ── Unshare plan ──────────────────────────────────────────────────────────────

func (a *Recipes) unsharePlanHandler(w http.ResponseWriter, r *http.Request) error {
	planID, err := parsePlanUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	user := currentUser(r)

	targetUserID := r.PathValue("userID")
	if targetUserID == "" {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Missing user",
		}
	}

	if err = a.services.Plans.Unshare(
		r.Context(), planID, user.ID, targetUserID,
	); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/plans/"+planID.String(), http.StatusSeeOther)
	return nil
}

// ── iCal feed (public, no auth) ───────────────────────────────────────────────

func (a *Recipes) icalFeedHandler(w http.ResponseWriter, r *http.Request) {
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

func buildCalendarDays(start, end time.Time, meals []models.PlanMeal) []planDay {
	mealsByDateSlot := make(map[string]*models.PlanMeal)
	for i := range meals {
		m := &meals[i]
		key := m.MealDate.Format("2006-01-02") + ":" + m.MealSlot
		mealsByDateSlot[key] = m
	}

	var days []planDay
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		//nolint:exhaustruct //other fields optional
		day := planDay{Date: d}
		if m, ok := mealsByDateSlot[dateStr+":breakfast"]; ok {
			day.Breakfast = m
		}
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
