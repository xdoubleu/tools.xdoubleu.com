import { create, isMessage } from '@bufbuild/protobuf'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import { toFraction } from '@/lib/recipes/fractions'
import {
  GetRecipeResponseSchema,
  IngredientSchema,
  ListRecipesResponseSchema,
  RecipeSchema,
  RecipesService,
  ScaledIngredientSchema,
  type CreateRecipeRequest,
  type GetRecipeResponse,
  type Recipe
} from '@/lib/gen/recipes/v1/recipes_pb'

const { method } = RecipesService
const REVALIDATE = '/recipes'

// Fields CreateRecipeRequest and UpdateRecipeRequest share.
type RecipeFields = Pick<
  CreateRecipeRequest,
  | 'name'
  | 'steps'
  | 'baseServings'
  | 'batchServings'
  | 'isDraft'
  | 'ingredientNames'
  | 'ingredientAmounts'
  | 'ingredientUnits'
  | 'ingredientGroupNames'
>

/** Builds the recipe the API would store for these fields (dtoToRecipe). */
function toRecipe(id: string, req: RecipeFields, base?: Recipe): Recipe {
  const ingredients = req.ingredientNames.flatMap((name, i) => {
    if (!name) return []
    const group = req.ingredientGroupNames[i]?.trim()
    return [
      create(IngredientSchema, {
        id: `${id}-${i}`,
        recipeId: id,
        name,
        amount: req.ingredientAmounts[i] ?? 0,
        unit: req.ingredientUnits[i]?.trim() ?? '',
        sortOrder: i,
        groupName: group || undefined
      })
    ]
  })
  return create(RecipeSchema, {
    id,
    userId: base?.userId,
    createdAt: base?.createdAt,
    updatedAt: base?.updatedAt,
    name: req.name,
    instructions: req.steps
      .map((s) => s.trim())
      .filter(Boolean)
      .join('\n'),
    baseServings: req.baseServings > 0 ? req.baseServings : 2,
    batchServings: req.batchServings,
    isDraft: req.isDraft,
    ingredients
  })
}

function isRecipeKey(key: unknown, id: string): boolean {
  const detail = swrKeys.recipe(id)
  return key === detail || (typeof key === 'string' && key.startsWith(`${detail}?servings=`))
}

/** Replaces a cached detail response's recipe, rescaling like GetRecipe. */
function withRecipe(data: GetRecipeResponse, recipe: Recipe): GetRecipeResponse {
  const servings = data.servings > 0 ? data.servings : recipe.baseServings
  const ratio = servings / recipe.baseServings
  return {
    ...data,
    recipe,
    servings,
    scaledIngredients: recipe.ingredients.map((ing) =>
      create(ScaledIngredientSchema, {
        name: ing.name,
        amount: toFraction(ing.amount * ratio),
        unit: ing.unit
      })
    )
  }
}

function upsertListed(list: Recipe[], recipe: Recipe): Recipe[] {
  return [...list.filter((r) => r.id !== recipe.id), recipe].sort(
    (a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id)
  )
}

export const createRecipeWrite = defineOfflineWrite({
  method: method.createRecipe,
  apply: (key, data, req) => {
    if (key !== swrKeys.recipes || !isMessage(data, ListRecipesResponseSchema)) return data
    return { ...data, recipes: upsertListed(data.recipes, toRecipe(req.id, req)) }
  },
  describe: (req) => `Add recipe “${req.name}”`,
  revalidate: REVALIDATE
})

export const updateRecipeWrite = defineOfflineWrite({
  method: method.updateRecipe,
  apply: (key, data, req) => {
    if (key === swrKeys.recipes && isMessage(data, ListRecipesResponseSchema)) {
      const existing = data.recipes.find((r) => r.id === req.id)
      if (!existing) return data
      return { ...data, recipes: upsertListed(data.recipes, toRecipe(req.id, req, existing)) }
    }
    if (isRecipeKey(key, req.id) && isMessage(data, GetRecipeResponseSchema) && data.recipe) {
      return withRecipe(data, toRecipe(req.id, req, data.recipe))
    }
    return data
  },
  describe: (req) => `Edit recipe “${req.name}”`,
  revalidate: REVALIDATE
})

export const deleteRecipeWrite = defineOfflineWrite({
  method: method.deleteRecipe,
  apply: (key, data, req) =>
    key === swrKeys.recipes && isMessage(data, ListRecipesResponseSchema)
      ? { ...data, recipes: data.recipes.filter((r) => r.id !== req.id) }
      : data,
  describe: () => 'Delete a recipe',
  revalidate: REVALIDATE
})

export const recipeWrites = [createRecipeWrite, updateRecipeWrite, deleteRecipeWrite]
