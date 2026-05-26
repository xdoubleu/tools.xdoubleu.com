import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { MealPlansService } from '@/lib/gen/mealplans/v1/mealplans_connect'
import type {
  ListPlansResponse,
  GetPlanResponse,
  AddMealRequest,
  DeleteMealRequest,
  MoveMealRequest,
  SharePlanRequest,
  UnsharePlanRequest,
  CreatePlanRequest,
  UpdatePlanRequest,
  DeletePlanRequest
} from '@/lib/gen/mealplans/v1/mealplans_pb'

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
