package recipes

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/contexttools"
	tpltools "github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func currentUser(r *http.Request) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
}

type scaledIngredient struct {
	Name   string
	Amount string
	Unit   string
}

// ── List recipes ──────────────────────────────────────────────────────────────

func (a *Recipes) listRecipesHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	recipeList, err := a.services.Recipes.List(r.Context(), user.ID)
	if err != nil {
		return err
	}
	tpltools.RenderWithPanic(a.Tpl, w, "recipes_list.html", map[string]any{
		"Recipes": recipeList,
	})
	return nil
}

// ── New recipe form ───────────────────────────────────────────────────────────

func (a *Recipes) newRecipeFormHandler(w http.ResponseWriter, _ *http.Request) error {
	tpltools.RenderWithPanic(a.Tpl, w, "recipes_form.html", map[string]any{
		//nolint:exhaustruct,mnd // other fields optional and no magic number
		"Recipe": models.Recipe{BaseServings: 2},
		"Action": "/recipes/new",
	})
	return nil
}

// ── Create recipe ─────────────────────────────────────────────────────────────

func (a *Recipes) createRecipeHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)

	var dto dtos.CreateRecipeDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	recipe, ingredients := dtoToRecipe(dto)
	recipe.Ingredients = ingredients

	if _, err := a.services.Recipes.Create(r.Context(), user.ID, recipe); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes", http.StatusSeeOther)
	return nil
}

// ── View or edit recipe (GET) ─────────────────────────────────────────────────
// GET /recipes/{id}        → view
// GET /recipes/{id}?edit=1 → edit form

func (a *Recipes) viewOrEditRecipeHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	id, err := parseUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Recipe not found",
		}
	}
	user := currentUser(r)

	recipe, err := a.services.Recipes.Get(r.Context(), id, user.ID)
	if err != nil {
		return err
	}

	if r.URL.Query().Get("edit") == "1" && recipe.UserID == user.ID {
		tpltools.RenderWithPanic(a.Tpl, w, "recipes_form.html", map[string]any{
			"Recipe": recipe,
			"Action": "/recipes/" + id.String(),
		})
		return nil
	}

	servings := recipe.BaseServings
	if s, parseErr := strconv.Atoi(r.URL.Query().Get("servings")); parseErr == nil &&
		s > 0 {
		servings = s
	}

	scaled := make([]scaledIngredient, len(recipe.Ingredients))
	for i, ing := range recipe.Ingredients {
		ratio := float64(servings) / float64(recipe.BaseServings)
		scaled[i] = scaledIngredient{
			Name:   ing.Name,
			Amount: toFraction(ing.Amount * ratio),
			Unit:   ing.Unit,
		}
	}

	tpltools.RenderWithPanic(a.Tpl, w, "recipes_view.html", map[string]any{
		"Recipe":   recipe,
		"Servings": servings,
		"Scaled":   scaled,
		"IsOwner":  recipe.UserID == user.ID,
	})
	return nil
}

// ── Update or delete recipe (POST) ────────────────────────────────────────────
// POST /recipes/{id} with _action=update → update
// POST /recipes/{id} with _action=delete → delete

func (a *Recipes) updateOrDeleteRecipeHandler(
	w http.ResponseWriter,
	r *http.Request,
) error {
	id, err := parseUUID(r)
	if err != nil {
		return &services.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Recipe not found",
		}
	}
	user := currentUser(r)

	const maxBodyBytes = 1 << 20 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err = r.ParseForm(); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	if r.FormValue("_action") == "delete" {
		if err = a.services.Recipes.Delete(r.Context(), id, user.ID); err != nil {
			return err
		}
		http.Redirect(w, r, "/recipes", http.StatusSeeOther)
		return nil
	}

	var dto dtos.CreateRecipeDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &services.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	recipe, ingredients := dtoToRecipe(dto)
	recipe.ID = id
	recipe.Ingredients = ingredients

	if err = a.services.Recipes.Update(r.Context(), user.ID, recipe); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/"+id.String(), http.StatusSeeOther)
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseUUID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}

func dtoToRecipe(dto dtos.CreateRecipeDto) (models.Recipe, []models.Ingredient) {
	//nolint:exhaustruct //other fields optional
	recipe := models.Recipe{
		Name:         dto.Name,
		Description:  dto.Description,
		BaseServings: dto.BaseServings,
		IsShared:     dto.IsShared,
	}
	if recipe.BaseServings <= 0 {
		recipe.BaseServings = 2
	}

	var ingredients []models.Ingredient
	for i := range dto.IngredientNames {
		name := dto.IngredientNames[i]
		if name == "" {
			continue
		}
		var amount float64
		if i < len(dto.IngredientAmounts) {
			if v, err := strconv.ParseFloat(dto.IngredientAmounts[i], 64); err == nil {
				amount = v
			}
		}
		unit := ""
		if i < len(dto.IngredientUnits) {
			unit = dto.IngredientUnits[i]
		}
		//nolint:exhaustruct // other fields optional
		ingredients = append(ingredients, models.Ingredient{
			Name:      name,
			Amount:    amount,
			Unit:      unit,
			SortOrder: i,
		})
	}
	return recipe, ingredients
}
