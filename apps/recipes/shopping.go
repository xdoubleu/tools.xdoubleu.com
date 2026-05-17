package recipes

import (
	"fmt"
	"net/http"
	"time"

	"tools.xdoubleu.com/apps/recipes/internal/services"
)

func (a *Recipes) shoppingListHandler(w http.ResponseWriter, r *http.Request) error {
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

	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)
	end := today.AddDate(0, 0, daysPerWeek-1)

	items, err := a.services.Shopping.GetList(r.Context(), id, today, end)
	if err != nil {
		return err
	}

	if r.URL.Query().Get("format") == "txt" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().
			Set("Content-Disposition", `attachment; filename="shopping-list.txt"`)
		for _, item := range items {
			_, _ = fmt.Fprintf(
				w,
				"%s %s %s\n",
				toFraction(item.Amount),
				item.Unit,
				item.Name,
			)
		}
		return nil
	}

	_ = PlansShoppingPage(PlansShoppingData{
		Plan:  *plan,
		Items: items,
	}).Render(r.Context(), w)
	return nil
}
