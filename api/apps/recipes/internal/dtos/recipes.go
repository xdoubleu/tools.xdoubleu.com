package dtos

type CreateRecipeDto struct {
	Name              string   `schema:"name"`
	Steps             []string `schema:"step"`
	BaseServings      int      `schema:"base_servings"`
	IngredientNames   []string `schema:"ingredient_name"`
	IngredientAmounts []string `schema:"ingredient_amount"`
	IngredientUnits   []string `schema:"ingredient_unit"`
}

type CreatePlanDto struct {
	Name string `schema:"name"`
}

type UpdatePlanDto struct {
	Name          string   `schema:"name"`
	ICalHideSlots []string `schema:"ical_hide_slots"`
	ICalHidePast  bool     `schema:"ical_hide_past"`
}

type AddMealDto struct {
	MealDate   string `schema:"meal_date"`
	MealSlot   string `schema:"meal_slot"`
	RecipeID   string `schema:"recipe_id"`
	CustomName string `schema:"custom_name"`
	Servings   int    `schema:"servings"`
}

type SharePlanDto struct {
	ContactUserID string `schema:"contact_user_id"`
	CanEdit       bool   `schema:"can_edit"`
}

type ShareRecipeDto struct {
	ContactUserID string `schema:"contact_user_id"`
}
