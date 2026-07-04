import useSWR from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  MealPlansService,
  UpdatePlanRequestSchema,
  CreateMealRequestSchema,
  DeleteMealRequestSchema,
  MoveMealRequestSchema,
  SharePlanRequestSchema,
  UnsharePlanRequestSchema
} from '@/lib/gen/mealplans/v1/mealplans_pb'
import type {
  ListPlansResponse,
  GetPlanResponse,
  SuggestRecipesResponse
} from '@/lib/gen/mealplans/v1/mealplans_pb'

export type UpdatePlanInput = MessageInitShape<typeof UpdatePlanRequestSchema>
export type AddMealInput = MessageInitShape<typeof CreateMealRequestSchema>
export type DeleteMealInput = MessageInitShape<typeof DeleteMealRequestSchema>
export type MoveMealInput = MessageInitShape<typeof MoveMealRequestSchema>
export type SharePlanInput = MessageInitShape<typeof SharePlanRequestSchema>
export type UnsharePlanInput = MessageInitShape<typeof UnsharePlanRequestSchema>

export function useMealPlans() {
  const client = createServiceClient(MealPlansService)
  return useSWR<ListPlansResponse, Error>(swrKeys.mealPlans, () => client.listPlans({}))
}

export function useMealPlan(id: string, offset: number = 0) {
  const client = createServiceClient(MealPlansService)
  return useSWR<GetPlanResponse, Error>(id ? swrKeys.mealPlan(id, offset) : null, () =>
    client.getPlan({ id, offset })
  )
}

// useMealSuggestions fetches recipe IDs previously planned on the same weekday
// and slot. The key is null (no fetch) until a cell is chosen, so it only runs
// while the add-entry form is open.
export function useMealSuggestions(planId: string, mealDate: string, mealSlot: string) {
  const client = createServiceClient(MealPlansService)
  return useSWR<SuggestRecipesResponse, Error>(
    planId && mealDate && mealSlot ? swrKeys.mealSuggestions(planId, mealDate, mealSlot) : null,
    () => client.suggestRecipes({ planId, mealDate, mealSlot })
  )
}

export function useUpdatePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: UpdatePlanInput) => client.updatePlan(req)
}

export function useAddMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: AddMealInput) => client.createMeal(req)
}

export function useDeleteMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: DeleteMealInput) => client.deleteMeal(req)
}

export function useMoveMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: MoveMealInput) => client.moveMeal(req)
}

export function useSharePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: SharePlanInput) => client.sharePlan(req)
}

export function useUnsharePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: UnsharePlanInput) => client.unsharePlan(req)
}
