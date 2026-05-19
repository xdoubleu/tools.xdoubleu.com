'use client'

import Link from 'next/link'
import {
  useMealPlan,
  useAddMeal,
  useDeleteMeal,
  useSharePlan,
  useUnsharePlan
} from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/recipes/MealPlanCalendar'
import {
  AddMealRequest,
  DeleteMealRequest,
  SharePlanRequest,
  UnsharePlanRequest
} from '@/lib/gen/recipes/v1/mealplans_pb'

export default function MealPlanPage({ params }: { params: { id: string } }) {
  const { data, error, isLoading, mutate } = useMealPlan(params.id)
  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()
  const sharePlan = useSharePlan()
  const unsharePlan = useUnsharePlan()

  const plan = data?.plan
  const recipes = data?.recipes ?? []

  const handleAddMeal = async (date: string, slot: string, recipeId: string, servings: number) => {
    if (!plan) return
    await addMeal(
      new AddMealRequest({
        planId: plan.id,
        mealDate: date,
        mealSlot: slot,
        recipeId,
        customName: '',
        servings
      })
    )
    await mutate()
  }

  const handleDeleteMeal = async (mealId: string) => {
    if (!plan) return
    await deleteMeal(new DeleteMealRequest({ planId: plan.id, mealId }))
    await mutate()
  }

  const handleShare = async () => {
    if (!plan) return
    const contactUserId = window.prompt('Enter contact user ID to share with:')
    if (!contactUserId) return
    await sharePlan(new SharePlanRequest({ planId: plan.id, contactUserId, canEdit: false }))
    await mutate()
  }

  const handleUnshare = async () => {
    if (!plan) return
    const targetUserId = window.prompt('Enter user ID to unshare with:')
    if (!targetUserId) return
    await unsharePlan(new UnsharePlanRequest({ planId: plan.id, targetUserId }))
    await mutate()
  }

  return (
    <main className="max-w-5xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/recipes/plans" className="text-blue-600 hover:underline text-sm">
          &larr; Meal Plans
        </Link>
      </div>

      {isLoading && <p>Loading meal plan...</p>}
      {error && <p className="text-red-600">Failed to load meal plan.</p>}

      {plan && (
        <>
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-3xl font-bold">{plan.name}</h1>
            <div className="flex gap-2">
              {data?.isOwner && (
                <>
                  <button
                    onClick={handleShare}
                    className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
                  >
                    Share
                  </button>
                  <button
                    onClick={handleUnshare}
                    className="px-4 py-2 bg-surface text-fg rounded hover:bg-border text-sm"
                  >
                    Unshare
                  </button>
                </>
              )}
            </div>
          </div>

          <MealPlanCalendar
            plan={plan}
            recipes={recipes}
            onAddMeal={handleAddMeal}
            onDeleteMeal={handleDeleteMeal}
          />
        </>
      )}
    </main>
  )
}
