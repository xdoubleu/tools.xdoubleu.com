import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_connect'
import { MealPlansService } from '@/lib/gen/recipes/v1/mealplans_connect'
import type {
  ListRecipesResponse,
  GetRecipeResponse,
  CreateRecipeRequest,
  UpdateRecipeRequest
} from '@/lib/gen/recipes/v1/recipes_pb'
import type {
  ListPlansResponse,
  GetPlanResponse,
  GetShoppingListResponse,
  AddMealRequest,
  DeleteMealRequest,
  SharePlanRequest,
  UnsharePlanRequest
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

export function useCreateRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: CreateRecipeRequest) => client.createRecipe(req)
}

export function useUpdateRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: UpdateRecipeRequest) => client.updateRecipe(req)
}

export function useAddMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: AddMealRequest) => client.addMeal(req)
}

export function useDeleteMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: DeleteMealRequest) => client.deleteMeal(req)
}

export function useSharePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: SharePlanRequest) => client.sharePlan(req)
}

export function useUnsharePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: UnsharePlanRequest) => client.unsharePlan(req)
}
