package dtos

type CreateRecipeDto struct {
	Name              string   `schema:"name"`
	Instructions      string   `schema:"instructions"`
	BaseServings      int      `schema:"base_servings"`
	IngredientNames   []string `schema:"ingredient_name"`
	IngredientAmounts []string `schema:"ingredient_amount"`
	IngredientUnits   []string `schema:"ingredient_unit"`
}

type CreatePlanDto struct {
	Name string `schema:"name"`
}

type AddMealDto struct {
	MealDate string `schema:"meal_date"`
	MealSlot string `schema:"meal_slot"`
	RecipeID string `schema:"recipe_id"`
	Servings int    `schema:"servings"`
}

type SharePlanDto struct {
	ContactUserID string `schema:"contact_user_id"`
	CanEdit       bool   `schema:"can_edit"`
}

type ShareRecipeDto struct {
	ContactUserID string `schema:"contact_user_id"`
}
