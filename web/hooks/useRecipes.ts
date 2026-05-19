import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_connect'
import { MealPlansService } from '@/lib/gen/recipes/v1/mealplans_connect'
import type { ListRecipesResponse, GetRecipeResponse } from '@/lib/gen/recipes/v1/recipes_pb'
import type {
  ListPlansResponse,
  GetPlanResponse,
  GetShoppingListResponse
} from '@/lib/gen/recipes/v1/mealplans_pb'

export function useRecipes() {
  const client = createServiceClient(RecipesService)
  return useSWR<ListRecipesResponse, Error>('/recipes', () => client.listRecipes({}))
}

export function useRecipe(id: string) {
  const client = createServiceClient(RecipesService)
  return useSWR<GetRecipeResponse, Error>(id ? `/recipes/${id}` : null, () =>
    client.getRecipe({ id })
  )
}

export function useMealPlans() {
  const client = createServiceClient(MealPlansService)
  return useSWR<ListPlansResponse, Error>('/recipes/plans', () => client.listPlans({}))
}

export function useMealPlan(id: string) {
  const client = createServiceClient(MealPlansService)
  return useSWR<GetPlanResponse, Error>(id ? `/recipes/plans/${id}` : null, () =>
    client.getPlan({ id, offset: 0 })
  )
}

export function useShoppingList(planId: string) {
  const client = createServiceClient(MealPlansService)
  return useSWR<GetShoppingListResponse, Error>(
    planId ? `/recipes/plans/${planId}/shopping` : null,
    () => client.getShoppingList({ planId })
  )
}
