package dtos

type CreateRecipeDto struct {
	Name              string   `schema:"name"`
	Description       string   `schema:"description"`
	BaseServings      int      `schema:"base_servings"`
	IsShared          bool     `schema:"is_shared"`
	IngredientNames   []string `schema:"ingredient_name"`
	IngredientAmounts []string `schema:"ingredient_amount"`
	IngredientUnits   []string `schema:"ingredient_unit"`
}

type CreatePlanDto struct {
	Name      string `schema:"name"`
	StartDate string `schema:"start_date"`
	EndDate   string `schema:"end_date"`
}

type AddMealDto struct {
	MealDate string `schema:"meal_date"`
	MealSlot string `schema:"meal_slot"`
	RecipeID string `schema:"recipe_id"`
	Servings int    `schema:"servings"`
}

type SharePlanDto struct {
	Email   string `schema:"email"`
	CanEdit bool   `schema:"can_edit"`
}
