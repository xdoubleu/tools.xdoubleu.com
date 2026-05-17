package recipes

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	httptools "github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"tools.xdoubleu.com/apps/recipes/internal/dtos"
	"tools.xdoubleu.com/apps/recipes/internal/models"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func currentUser(r *http.Request) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		r.Context(),
		constants.UserContextKey,
	)
}

type stepEntry struct {
	N    int
	Text string
}

// ── List recipes ──────────────────────────────────────────────────────────────

func (a *Recipes) listRecipesHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)
	recipeList, err := a.services.Recipes.List(r.Context(), user.ID)
	if err != nil {
		return err
	}
	_ = RecipesListPage(RecipesListData{Recipes: recipeList}).Render(r.Context(), w)
	return nil
}

// ── New recipe form ───────────────────────────────────────────────────────────

func (a *Recipes) newRecipeFormHandler(w http.ResponseWriter, r *http.Request) error {
	//nolint:exhaustruct,mnd // other fields optional and no magic number
	_ = RecipesFormPage(RecipesFormData{
		Recipe: models.Recipe{BaseServings: 2},
		Steps:  []stepEntry{},
		Action: "/recipes/new",
		IsEdit: false,
	}).Render(r.Context(), w)
	return nil
}

// ── Create recipe ─────────────────────────────────────────────────────────────

func (a *Recipes) createRecipeHandler(w http.ResponseWriter, r *http.Request) error {
	user := currentUser(r)

	var dto dtos.CreateRecipeDto
	if err := httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	recipe, ingredients := dtoToRecipe(dto)
	recipe.Ingredients = ingredients

	if _, err := a.services.Recipes.Create(r.Context(), user.ID, recipe); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/list", http.StatusSeeOther)
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
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Recipe not found",
		}
	}
	user := currentUser(r)

	recipe, err := a.services.Recipes.Get(r.Context(), id, user.ID)
	if err != nil {
		return err
	}

	steps := splitSteps(recipe.Instructions)

	if r.URL.Query().Get("edit") == "1" && recipe.UserID == user.ID {
		_ = RecipesFormPage(RecipesFormData{
			Recipe: *recipe,
			Steps:  steps,
			Action: "/recipes/" + id.String(),
			IsEdit: true,
		}).Render(r.Context(), w)
		return nil
	}

	servings := recipe.BaseServings
	if s, parseErr := strconv.Atoi(r.URL.Query().Get("servings")); parseErr == nil &&
		s > 0 {
		servings = s
	}

	scaled := make([]ScaledIngredient, len(recipe.Ingredients))
	for i, ing := range recipe.Ingredients {
		ratio := float64(servings) / float64(recipe.BaseServings)
		scaled[i] = ScaledIngredient{
			Name:   ing.Name,
			Amount: toFraction(ing.Amount * ratio),
			Unit:   ing.Unit,
		}
	}

	contacts, err := a.contacts.List(r.Context(), user.ID)
	if err != nil {
		return err
	}

	_ = RecipesViewPage(RecipesViewData{
		Recipe:   *recipe,
		Steps:    steps,
		Servings: servings,
		Scaled:   scaled,
		IsOwner:  recipe.UserID == user.ID,
		Contacts: contacts,
	}).Render(r.Context(), w)
	return nil
}

func splitSteps(instructions string) []stepEntry {
	if instructions == "" {
		return nil
	}
	var steps []stepEntry
	n := 1
	for _, s := range strings.Split(instructions, "\n") {
		if t := strings.TrimSpace(s); t != "" {
			steps = append(steps, stepEntry{N: n, Text: t})
			n++
		}
	}
	return steps
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
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Recipe not found",
		}
	}
	user := currentUser(r)

	const maxBodyBytes = 1 << 20 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err = r.ParseForm(); err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	if r.FormValue("_action") == "delete" {
		if err = a.services.Recipes.Delete(r.Context(), id, user.ID); err != nil {
			return err
		}
		http.Redirect(w, r, "/recipes/list", http.StatusSeeOther)
		return nil
	}

	var dto dtos.CreateRecipeDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
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

// ── Share recipe ──────────────────────────────────────────────────────────────

func (a *Recipes) shareRecipeHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := parseUUID(r)
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Recipe not found",
		}
	}
	user := currentUser(r)

	var dto dtos.ShareRecipeDto
	if err = httptools.ReadForm(r, &dto); err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid form data",
		}
	}

	if err = a.services.Recipes.Share(
		r.Context(), id, user.ID, dto.ContactUserID,
	); err != nil {
		return err
	}

	http.Redirect(w, r, "/recipes/"+id.String(), http.StatusSeeOther)
	return nil
}

// ── Unshare recipe ────────────────────────────────────────────────────────────

func (a *Recipes) unshareRecipeHandler(w http.ResponseWriter, r *http.Request) error {
	id, err := parseUUID(r)
	if err != nil {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Recipe not found",
		}
	}
	user := currentUser(r)

	targetUserID := r.PathValue("userID")
	if targetUserID == "" {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Missing user",
		}
	}

	if err = a.services.Recipes.Unshare(
		r.Context(), id, user.ID, targetUserID,
	); err != nil {
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
	var nonEmpty []string
	for _, s := range dto.Steps {
		if t := strings.TrimSpace(s); t != "" {
			nonEmpty = append(nonEmpty, t)
		}
	}

	//nolint:exhaustruct //other fields optional
	recipe := models.Recipe{
		Name:         dto.Name,
		Instructions: strings.Join(nonEmpty, "\n"),
		BaseServings: dto.BaseServings,
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
			if v, err := strconv.ParseFloat(
				strings.TrimSpace(dto.IngredientAmounts[i]), 64,
			); err == nil {
				amount = v
			}
		}
		unit := ""
		if i < len(dto.IngredientUnits) {
			unit = strings.TrimSpace(dto.IngredientUnits[i])
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
