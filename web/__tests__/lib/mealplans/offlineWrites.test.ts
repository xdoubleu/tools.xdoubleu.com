import { create, isMessage, type DescMessage, type MessageInitShape } from '@bufbuild/protobuf'
import type { OfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import { GetPlanResponseSchema, ListPlansResponseSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import {
  createMealWrite,
  deleteMealWrite,
  mealPlanWrites,
  moveMealWrite,
  updateMealWrite,
  updatePlanWrite
} from '@/lib/mealplans/offlineWrites'

function run<I extends DescMessage>(
  write: OfflineWrite<I>,
  init: MessageInitShape<I>,
  key: unknown,
  data: unknown,
  hint?: unknown
) {
  return write.apply(key, data, write.encode(init), hint)
}

const key = swrKeys.mealPlan('p1', 0)

const detail = () =>
  create(GetPlanResponseSchema, {
    plan: {
      id: 'p1',
      name: 'Week',
      meals: [
        {
          id: 'm1',
          planId: 'p1',
          mealDate: '2024-01-22',
          mealSlot: 'noon',
          recipeId: 'r1',
          recipe: { id: 'r1', name: 'Soup' },
          servings: 2
        },
        { id: 'm2', planId: 'p1', mealDate: '2024-01-23', mealSlot: 'evening', customName: 'Pizza' }
      ]
    },
    isOwner: true,
    windowStart: '2024-01-22T00:00:00Z',
    windowEnd: '2024-01-28T00:00:00Z'
  })

const meals = (data: unknown) => (isMessage(data, GetPlanResponseSchema) ? data.plan?.meals : [])

describe('meal plan offline writes', () => {
  it('leave unrelated keys and data untouched', () => {
    const data = detail()
    for (const write of mealPlanWrites) {
      expect(write.apply('/recipes', data, new Uint8Array(), undefined)).toBe(data)
    }
    expect(run(deleteMealWrite, { planId: 'p1', mealId: 'm1' }, key, 'nope')).toBe('nope')
    expect(
      run(deleteMealWrite, { planId: 'p1', mealId: 'm1' }, swrKeys.mealPlan('p2', 0), data)
    ).toBe(data)
    const empty = create(GetPlanResponseSchema, {})
    expect(run(deleteMealWrite, { planId: 'p1', mealId: 'm1' }, key, empty)).toBe(empty)
  })

  it('adds a created meal in the window with the default servings and recipe name', () => {
    const req = { id: 'm3', planId: 'p1', mealDate: '2024-01-28', mealSlot: 'noon', recipeId: 'r2' }
    const result = run(createMealWrite, req, key, detail(), 'Stew')

    expect(meals(result)).toHaveLength(3)
    expect(meals(result)?.[2]).toMatchObject({
      id: 'm3',
      mealDate: '2024-01-28',
      servings: 2,
      recipe: { id: 'r2', name: 'Stew' }
    })
    // Re-applied over a refetch that already holds it, it stays one meal.
    expect(meals(run(createMealWrite, req, key, result, 'Stew'))).toHaveLength(3)
  })

  it('keeps a created custom meal without a recipe and skips other weeks', () => {
    const custom = run(
      createMealWrite,
      { id: 'm3', planId: 'p1', mealDate: '2024-01-22', customName: 'Toast', servings: 1 },
      key,
      detail()
    )
    expect(meals(custom)?.[2]).toMatchObject({ customName: 'Toast', servings: 1 })
    expect(meals(custom)?.[2].recipe).toBeUndefined()

    for (const mealDate of ['2024-01-21', '2024-01-29']) {
      const result = run(createMealWrite, { id: 'm3', planId: 'p1', mealDate }, key, detail())
      expect(meals(result)).toHaveLength(2)
    }
  })

  it('updates a meal, keeping its recipe unless the recipe changed', () => {
    const same = run(
      updateMealWrite,
      { planId: 'p1', mealId: 'm1', recipeId: 'r1', servings: 4 },
      key,
      detail()
    )
    expect(meals(same)?.[0]).toMatchObject({ servings: 4, recipe: { name: 'Soup' } })
    expect(meals(same)?.[1]).toMatchObject({ id: 'm2', customName: 'Pizza' })

    const renamed = run(
      updateMealWrite,
      { planId: 'p1', mealId: 'm1', recipeId: 'r2', excludeFromShoppingList: true },
      key,
      detail(),
      'Stew'
    )
    expect(meals(renamed)?.[0]).toMatchObject({
      recipeId: 'r2',
      servings: 2,
      excludeFromShoppingList: true,
      recipe: { id: 'r2', name: 'Stew' }
    })

    const unknown = run(
      updateMealWrite,
      { planId: 'p1', mealId: 'm1', recipeId: 'r2' },
      key,
      detail()
    )
    expect(meals(unknown)?.[0].recipe).toBeUndefined()

    const custom = run(
      updateMealWrite,
      { planId: 'p1', mealId: 'm1', customName: 'Leftovers' },
      key,
      detail()
    )
    expect(meals(custom)?.[0]).toMatchObject({ recipeId: '', customName: 'Leftovers' })
    expect(meals(custom)?.[0].recipe).toBeUndefined()
  })

  it('removes a deleted meal', () => {
    const result = run(deleteMealWrite, { planId: 'p1', mealId: 'm1' }, key, detail())
    expect(meals(result)?.map((m) => m.id)).toEqual(['m2'])
  })

  it('moves a meal, dropping it when it leaves the week', () => {
    const moved = run(
      moveMealWrite,
      { planId: 'p1', mealId: 'm1', newDate: '2024-01-24', newSlot: 'breakfast' },
      key,
      detail()
    )
    expect(meals(moved)?.[0]).toMatchObject({
      id: 'm1',
      mealDate: '2024-01-24',
      mealSlot: 'breakfast'
    })
    expect(meals(moved)?.[1]).toMatchObject({ id: 'm2', mealDate: '2024-01-23' })

    const gone = run(
      moveMealWrite,
      { planId: 'p1', mealId: 'm1', newDate: '2024-02-01', newSlot: 'noon' },
      key,
      detail()
    )
    expect(meals(gone)?.map((m) => m.id)).toEqual(['m2'])
  })

  it('updates the plan in the list and in its detail responses', () => {
    const req = { id: 'p1', name: 'Next week', icalHideSlots: ['noon'], icalHidePast: true }
    const list = create(ListPlansResponseSchema, {
      plans: [
        { id: 'p1', name: 'Week' },
        { id: 'p2', name: 'Other' }
      ]
    })

    expect(run(updatePlanWrite, req, swrKeys.mealPlans, list)).toMatchObject({
      plans: [
        { id: 'p1', name: 'Next week', icalHideSlots: ['noon'], icalHidePast: true },
        { name: 'Other' }
      ]
    })
    const result = run(updatePlanWrite, req, swrKeys.mealPlan('p1', 3), detail())
    expect(result).toMatchObject({
      plan: { name: 'Next week', icalHidePast: true, meals: [{}, {}] }
    })

    const data = detail()
    expect(run(updatePlanWrite, req, swrKeys.mealPlan('p2', 0), data)).toBe(data)
    expect(run(updatePlanWrite, req, swrKeys.mealPlans, data)).toBe(data)
    const empty = create(GetPlanResponseSchema, {})
    expect(run(updatePlanWrite, req, key, empty)).toBe(empty)
  })

  it('ignores tuple keys, list data under other keys and stray hints', () => {
    const data = detail()
    expect(run(deleteMealWrite, { planId: 'p1', mealId: 'm1' }, [key], data)).toBe(data)
    const list = create(ListPlansResponseSchema, { plans: [{ id: 'p1', name: 'Week' }] })
    expect(run(updatePlanWrite, { id: 'p1', name: 'X' }, '/mealplans/other', list)).toBe(list)

    const custom = run(
      createMealWrite,
      { id: 'm3', planId: 'p1', mealDate: '2024-01-22', customName: 'Toast' },
      key,
      detail(),
      'Stale name'
    )
    expect(meals(custom)?.[2].recipe).toBeUndefined()
    const unnamed = run(
      createMealWrite,
      { id: 'm3', planId: 'p1', mealDate: '2024-01-22', recipeId: 'r9' },
      key,
      detail()
    )
    expect(meals(unnamed)?.[2]).toMatchObject({ recipeId: 'r9' })
    expect(meals(unnamed)?.[2].recipe).toBeUndefined()
  })

  it('describes each write', () => {
    expect(createMealWrite.describe(createMealWrite.encode({ mealDate: '2024-01-22' }))).toBe(
      'Plan a meal on 2024-01-22'
    )
    expect(updateMealWrite.describe(updateMealWrite.encode({}))).toBe('Edit a planned meal')
    expect(deleteMealWrite.describe(deleteMealWrite.encode({}))).toBe('Remove a planned meal')
    expect(moveMealWrite.describe(moveMealWrite.encode({ newDate: '2024-01-24' }))).toBe(
      'Move a meal to 2024-01-24'
    )
    expect(updatePlanWrite.describe(updatePlanWrite.encode({ name: 'Week' }))).toBe(
      'Edit plan “Week”'
    )
  })

  it('refetches meal plans and registers every write', () => {
    expect(mealPlanWrites).toEqual([
      createMealWrite,
      updateMealWrite,
      deleteMealWrite,
      moveMealWrite,
      updatePlanWrite
    ])
    for (const write of mealPlanWrites) expect(write.revalidate).toBe('/mealplans')
  })
})
