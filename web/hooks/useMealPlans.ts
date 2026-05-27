import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  MealPlansService,
  CreatePlanRequestSchema,
  UpdatePlanRequestSchema,
  DeletePlanRequestSchema,
  AddMealRequestSchema,
  DeleteMealRequestSchema,
  MoveMealRequestSchema,
  SharePlanRequestSchema,
  UnsharePlanRequestSchema
} from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { ListPlansResponse, GetPlanResponse } from '@/lib/gen/mealplans/v1/mealplans_pb'

export type CreatePlanInput = MessageInitShape<typeof CreatePlanRequestSchema>
export type UpdatePlanInput = MessageInitShape<typeof UpdatePlanRequestSchema>
export type DeletePlanInput = MessageInitShape<typeof DeletePlanRequestSchema>
export type AddMealInput = MessageInitShape<typeof AddMealRequestSchema>
export type DeleteMealInput = MessageInitShape<typeof DeleteMealRequestSchema>
export type MoveMealInput = MessageInitShape<typeof MoveMealRequestSchema>
export type SharePlanInput = MessageInitShape<typeof SharePlanRequestSchema>
export type UnsharePlanInput = MessageInitShape<typeof UnsharePlanRequestSchema>

export function useMealPlans() {
  const client = createServiceClient(MealPlansService)
  return useSWR<ListPlansResponse, Error>('/mealplans', () => client.listPlans({}))
}

export function useMealPlan(id: string, offset: number = 0) {
  const client = createServiceClient(MealPlansService)
  return useSWR<GetPlanResponse, Error>(id ? `/mealplans/${id}?offset=${offset}` : null, () =>
    client.getPlan({ id, offset })
  )
}

export function useCreatePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: CreatePlanInput) => client.createPlan(req)
}

export function useUpdatePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: UpdatePlanInput) => client.updatePlan(req)
}

export function useDeletePlan() {
  const client = createServiceClient(MealPlansService)
  return (req: DeletePlanInput) => client.deletePlan(req)
}

export function useAddMeal() {
  const client = createServiceClient(MealPlansService)
  return (req: AddMealInput) => client.addMeal(req)
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
