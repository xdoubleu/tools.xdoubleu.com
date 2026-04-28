package recipes

import (
	"fmt"
	"net/http"
)

func (a *Recipes) Routes(prefix string, mux *http.ServeMux) {
	p := fmt.Sprintf("/%s", prefix)

	// ── Recipe routes ──────────────────────────────────────────────────────────
	// All recipe item mutations use POST /recipes/{id} with a _action field to
	// avoid depth-3 route conflicts with /recipes/plans/{id}.
	// /recipes redirects to plans (primary feature); /recipes/list shows all recipes.
	mux.HandleFunc(
		fmt.Sprintf("GET %s", p),
		a.services.Auth.AppAccess(prefix, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, p+"/plans", http.StatusSeeOther)
		}),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/list", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.listRecipesHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/new", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.newRecipeFormHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/new", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.createRecipeHandler)),
	)
	// GET /recipes/{id}?edit=1 serves the edit form; without it serves the view.
	mux.HandleFunc(
		fmt.Sprintf("GET %s/{id}", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.viewOrEditRecipeHandler)),
	)
	// POST /recipes/{id} handles both update (_action=update) and delete (_action=delete).
	mux.HandleFunc(
		fmt.Sprintf("POST %s/{id}", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.updateOrDeleteRecipeHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/{id}/share", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.shareRecipeHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/{id}/share/{userID}/delete", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.unshareRecipeHandler)),
	)

	// ── Plan routes ────────────────────────────────────────────────────────────
	// Public iCal feed: /recipes/ical/<token>.ics
	// Depth-2 subtree avoids all conflicts with /recipes/plans/* routes.
	mux.HandleFunc(
		fmt.Sprintf("GET %s/ical/", p),
		a.icalFeedHandler,
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/plans", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.listPlansHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/plans/new", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.newPlanFormHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/plans/new", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.createPlanHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/plans/{id}", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.viewPlanHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/plans/{id}/edit", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.editPlanFormHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/plans/{id}/edit", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.updatePlanHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/plans/{id}/delete", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.deletePlanHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/plans/{id}/meals", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.addMealHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/plans/{id}/meals/{mealId}/delete", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.deleteMealHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("GET %s/plans/{id}/shopping", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.shoppingListHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/plans/{id}/share", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.sharePlanHandler)),
	)
	mux.HandleFunc(
		fmt.Sprintf("POST %s/plans/{id}/share/{userID}/delete", p),
		a.services.Auth.AppAccess(prefix, a.handle(a.unsharePlanHandler)),
	)
}
