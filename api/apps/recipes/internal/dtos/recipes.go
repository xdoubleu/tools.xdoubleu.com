package dtos

type CreateRecipeDto struct {
	Name              string   `schema:"name"`
	Steps             []string `schema:"step"`
	BaseServings      int      `schema:"base_servings"`
	IngredientNames   []string `schema:"ingredient_name"`
	IngredientAmounts []string `schema:"ingredient_amount"`
	IngredientUnits   []string `schema:"ingredient_unit"`
}

type ShareRecipeDto struct {
	ContactUserID string `schema:"contact_user_id"`
}
