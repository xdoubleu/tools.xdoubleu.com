import { create, isMessage } from '@bufbuild/protobuf'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  GetPlanResponseSchema,
  ListPlansResponseSchema,
  MealPlansService,
  PlanMealSchema,
  type GetPlanResponse,
  type PlanMeal
} from '@/lib/gen/mealplans/v1/mealplans_pb'
import { RecipeSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const { method } = MealPlansService
const REVALIDATE = '/mealplans'

// Meal writes' hint is the chosen recipe's name, which requests don't carry.
function recipeRef(recipeId: string, hint: unknown, current?: PlanMeal) {
  if (!recipeId) return undefined
  if (typeof hint === 'string') return create(RecipeSchema, { id: recipeId, name: hint })
  return current?.recipeId === recipeId ? current.recipe : undefined
}

/** Whether the date (YYYY-MM-DD) falls in the response's week window. */
function inWindow(data: GetPlanResponse, date: string): boolean {
  return date >= data.windowStart.slice(0, 10) && date <= data.windowEnd.slice(0, 10)
}

/** Whether `key` holds `planId`'s GetPlan response, any week offset. */
function isPlanKey(key: unknown, planId: string): boolean {
  return typeof key === 'string' && key.startsWith(`${swrKeys.mealPlans}/${planId}?offset=`)
}

/** Runs `fn` on the meals of a cached GetPlan response for `planId`. */
function onMeals(
  key: unknown,
  data: unknown,
  planId: string,
  fn: (data: GetPlanResponse, meals: PlanMeal[]) => PlanMeal[]
): unknown {
  if (!isPlanKey(key, planId) || !isMessage(data, GetPlanResponseSchema) || !data.plan) {
    return data
  }
  return { ...data, plan: { ...data.plan, meals: fn(data, data.plan.meals) } }
}

const servingsOrDefault = (servings: number) => (servings > 0 ? servings : 2)

export const createMealWrite = defineOfflineWrite({
  method: method.createMeal,
  apply: (key, data, req, hint) =>
    onMeals(key, data, req.planId, (d, meals) => {
      if (!inWindow(d, req.mealDate)) return meals
      const meal = create(PlanMealSchema, {
        id: req.id,
        planId: req.planId,
        mealDate: req.mealDate,
        mealSlot: req.mealSlot,
        recipeId: req.recipeId,
        customName: req.customName,
        servings: servingsOrDefault(req.servings),
        recipe: recipeRef(req.recipeId, hint),
        excludeFromShoppingList: req.excludeFromShoppingList
      })
      return [...meals.filter((m) => m.id !== req.id), meal]
    }),
  describe: (req) => `Plan a meal on ${req.mealDate}`,
  revalidate: REVALIDATE
})

export const updateMealWrite = defineOfflineWrite({
  method: method.updateMeal,
  apply: (key, data, req, hint) =>
    onMeals(key, data, req.planId, (_, meals) =>
      meals.map((m) =>
        m.id === req.mealId
          ? {
              ...m,
              recipeId: req.recipeId,
              customName: req.customName,
              servings: servingsOrDefault(req.servings),
              recipe: recipeRef(req.recipeId, hint, m),
              excludeFromShoppingList: req.excludeFromShoppingList
            }
          : m
      )
    ),
  describe: () => 'Edit a planned meal',
  revalidate: REVALIDATE
})

export const deleteMealWrite = defineOfflineWrite({
  method: method.deleteMeal,
  apply: (key, data, req) =>
    onMeals(key, data, req.planId, (_, meals) => meals.filter((m) => m.id !== req.mealId)),
  describe: () => 'Remove a planned meal',
  revalidate: REVALIDATE
})

export const moveMealWrite = defineOfflineWrite({
  method: method.moveMeal,
  apply: (key, data, req) =>
    onMeals(key, data, req.planId, (d, meals) =>
      meals.flatMap((m) => {
        if (m.id !== req.mealId) return [m]
        if (!inWindow(d, req.newDate)) return []
        return [{ ...m, mealDate: req.newDate, mealSlot: req.newSlot }]
      })
    ),
  describe: (req) => `Move a meal to ${req.newDate}`,
  revalidate: REVALIDATE
})

export const updatePlanWrite = defineOfflineWrite({
  method: method.updatePlan,
  apply: (key, data, req) => {
    const fields = {
      name: req.name,
      icalHideSlots: req.icalHideSlots,
      icalHidePast: req.icalHidePast
    }
    if (key === swrKeys.mealPlans && isMessage(data, ListPlansResponseSchema)) {
      return { ...data, plans: data.plans.map((p) => (p.id === req.id ? { ...p, ...fields } : p)) }
    }
    if (isPlanKey(key, req.id) && isMessage(data, GetPlanResponseSchema) && data.plan) {
      return { ...data, plan: { ...data.plan, ...fields } }
    }
    return data
  },
  describe: (req) => `Edit plan “${req.name}”`,
  revalidate: REVALIDATE
})

export const mealPlanWrites = [
  createMealWrite,
  updateMealWrite,
  deleteMealWrite,
  moveMealWrite,
  updatePlanWrite
]
