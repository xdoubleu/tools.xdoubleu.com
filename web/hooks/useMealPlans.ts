import useSWR from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import { enqueueWrite, flushOutbox, sendWrite } from '@/lib/offline/outbox'
import {
  createMealWrite,
  deleteMealWrite,
  moveMealWrite,
  updateMealWrite,
  updatePlanWrite
} from '@/lib/mealplans/offlineWrites'
import {
  MealPlansService,
  UpdatePlanRequestSchema,
  CreateMealRequestSchema,
  UpdateMealRequestSchema,
  DeleteMealRequestSchema,
  MoveMealRequestSchema
} from '@/lib/gen/mealplans/v1/mealplans_pb'
import type {
  ListPlansResponse,
  GetPlanResponse,
  SuggestRecipesResponse
} from '@/lib/gen/mealplans/v1/mealplans_pb'

export type UpdatePlanInput = MessageInitShape<typeof UpdatePlanRequestSchema>
export type AddMealInput = MessageInitShape<typeof CreateMealRequestSchema>
export type UpdateMealInput = MessageInitShape<typeof UpdateMealRequestSchema>
export type DeleteMealInput = MessageInitShape<typeof DeleteMealRequestSchema>
export type MoveMealInput = MessageInitShape<typeof MoveMealRequestSchema>

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

// useMealSuggestions fetches recipes previously planned on the same weekday
// and slot; null key until a cell is chosen.
export function useMealSuggestions(planId: string, mealDate: string, mealSlot: string) {
  const client = createServiceClient(MealPlansService)
  return useSWR<SuggestRecipesResponse, Error>(
    planId && mealDate && mealSlot ? swrKeys.mealSuggestions(planId, mealDate, mealSlot) : null,
    () => client.suggestRecipes({ planId, mealDate, mealSlot })
  )
}

export function useRotateICalToken() {
  const client = createServiceClient(MealPlansService)
  return (id: string) => client.rotateICalToken({ id })
}

// Writes go through the offline outbox: they show at once and wait for the
// send attempt. A rejected meal write rolls back and shows in the offline
// banner; a rejected plan edit throws.

export function useUpdatePlan() {
  return async (req: UpdatePlanInput) => {
    await sendWrite(enqueueWrite(updatePlanWrite, req))
  }
}

/** Queues a meal with a client ID; `recipeName` labels it until it syncs. */
export function useAddMeal() {
  return async (req: AddMealInput, recipeName?: string) => {
    const msg = create(CreateMealRequestSchema, req)
    msg.id = crypto.randomUUID()
    await enqueueWrite(createMealWrite, msg, recipeName)
    await flushOutbox()
  }
}

export function useUpdateMeal() {
  return async (req: UpdateMealInput, recipeName?: string) => {
    await enqueueWrite(updateMealWrite, req, recipeName)
    await flushOutbox()
  }
}

export function useDeleteMeal() {
  return async (req: DeleteMealInput) => {
    await enqueueWrite(deleteMealWrite, req)
    await flushOutbox()
  }
}

export function useMoveMeal() {
  return async (req: MoveMealInput) => {
    await enqueueWrite(moveMealWrite, req)
    await flushOutbox()
  }
}
