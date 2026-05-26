import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_connect'
import { MealPlansService } from '@/lib/gen/recipes/v1/mealplans_connect'
import type {
  ListRecipesResponse,
  GetRecipeResponse,
  CreateRecipeRequest,
  UpdateRecipeRequest,
  DeleteRecipeRequest,
  ShareRecipeRequest,
  UnshareRecipeRequest
} from '@/lib/gen/recipes/v1/recipes_pb'
import type {
  ListPlansResponse,
  GetPlanResponse,
  GetShoppingListResponse,
  AddMealRequest,
  DeleteMealRequest,
  MoveMealRequest,
  SharePlanRequest,
  UnsharePlanRequest,
  CreatePlanRequest,
  UpdatePlanRequest,
  DeletePlanRequest
} from '@/lib/gen/recipes/v1/mealplans_pb'

export function useRecipes() {
  const client = createServiceClient(RecipesService)
  return useSWR<ListRecipesResponse, Error>('/recipes', () => client.listRecipes({}))
}

export function useRecipe(id: string, servings?: number) {
  const client = createServiceClient(RecipesService)
  const key = id ? (servings ? `/recipes/${id}?servings=${servings}` : `/recipes/${id}`) : null
  return useSWR<GetRecipeResponse, Error>(key, () =>
    client.getRecipe({ id, servings: servings ?? 0 })
  )
}

export function useMealPlans() {
  const client = createServiceClient(MealPlansService)
  return useSWR<ListPlansResponse, Error>('/recipes/plans', () => client.listPlans({}))
}

export function useMealPlan(id: string, offset: number = 0) {
  const client = createServiceClient(MealPlansService)
  return useSWR<GetPlanResponse, Error>(id ? `/recipes/plans/${id}?offset=${offset}` : null, () =>
    client.getPlan({ id, offset })
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

export function useDeleteRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: DeleteRecipeRequest) => client.deleteRecipe(req)
}

export function useShareRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: ShareRecipeRequest) => client.shareRecipe(req)
}

export function useUnshareRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: UnshareRecipeRequest) => client.unshareRecipe(req)
}

export function useCreatePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: CreatePlanRequest) => client.createPlan(req)
}

export function useUpdatePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: UpdatePlanRequest) => client.updatePlan(req)
}

export function useDeletePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: DeletePlanRequest) => client.deletePlan(req)
}

export function useAddMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: AddMealRequest) => client.addMeal(req)
}

export function useDeleteMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: DeleteMealRequest) => client.deleteMeal(req)
}

export function useMoveMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: MoveMealRequest) => client.moveMeal(req)
}

export function useSharePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: SharePlanRequest) => client.sharePlan(req)
}

export function useUnsharePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: UnsharePlanRequest) => client.unsharePlan(req)
}
