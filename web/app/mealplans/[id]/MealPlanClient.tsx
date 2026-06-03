'use client'

import { useState } from 'react'
import Link from 'next/link'
import { getApiUrl } from '@/lib/env'
import {
  useMealPlan,
  useAddMeal,
  useDeleteMeal,
  useSharePlan,
  useUnsharePlan
} from '@/hooks/useMealPlans'
import type {
  AddMealInput,
  DeleteMealInput,
  SharePlanInput,
  UnsharePlanInput
} from '@/hooks/useMealPlans'
import { useRecipes } from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/recipes/MealPlanCalendar'
import ShareModal from '@/components/recipes/ShareModal'

export default function MealPlanClient({ id }: { id: string }) {
  const [offset, setOffset] = useState(0)
  const { data, error, isLoading, mutate } = useMealPlan(id, offset)
  const { data: recipesData } = useRecipes()
  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()
  const sharePlan = useSharePlan()
  const unsharePlan = useUnsharePlan()

  const [showShareModal, setShowShareModal] = useState(false)
  const [icalCopied, setIcalCopied] = useState(false)

  const handleCopyIcal = () => {
    if (!data?.icalUrl) return
    const url = `${getApiUrl()}${data.icalUrl}`
    navigator.clipboard.writeText(url).then(() => {
      setIcalCopied(true)
      setTimeout(() => setIcalCopied(false), 2000)
    })
  }

  const plan = data?.plan
  const recipes = recipesData?.recipes ?? []
  const isOwner = data?.isOwner ?? false

  const handleAddMeal = async (
    date: string,
    slot: string,
    recipeId: string,
    customName: string,
    servings: number
  ) => {
    if (!plan) return
    const req: AddMealInput = {
      planId: plan.id,
      mealDate: date,
      mealSlot: slot,
      recipeId,
      customName,
      servings
    }
    await addMeal(req)
    await mutate()
  }

  const handleDeleteMeal = async (mealId: string) => {
    if (!plan) return
    const req: DeleteMealInput = { planId: plan.id, mealId }
    await deleteMeal(req)
    await mutate()
  }

  const handleShare = async (userId: string) => {
    if (!plan) return
    const req: SharePlanInput = { planId: plan.id, contactUserId: userId, canEdit: false }
    await sharePlan(req)
    await mutate()
  }

  const handleUnshare = async (userId: string) => {
    if (!plan) return
    const req: UnsharePlanInput = { planId: plan.id, targetUserId: userId }
    await unsharePlan(req)
    await mutate()
  }

  return (
    <main className="max-w-5xl mx-auto p-6">
      {isLoading && <p>Loading meal plan...</p>}
      {error && <p className="text-danger">Failed to load meal plan.</p>}

      {plan && (
        <>
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-3xl font-bold">{plan.name}</h1>
            <div className="flex gap-2">
              {data?.icalUrl && (
                <button
                  onClick={handleCopyIcal}
                  className="px-4 py-2 bg-surface border border-border rounded-xl hover:bg-border text-sm"
                >
                  {icalCopied ? 'Copied!' : 'iCal Link'}
                </button>
              )}
              {isOwner && (
                <>
                  <button
                    onClick={() => setShowShareModal(true)}
                    className="px-4 py-2 bg-surface border border-border rounded-xl hover:bg-border text-sm"
                  >
                    Share
                  </button>
                  <Link
                    href={`/mealplans/${plan.id}/edit`}
                    className="px-4 py-2 bg-surface border border-border rounded-xl hover:bg-border text-sm"
                  >
                    Settings
                  </Link>
                </>
              )}
            </div>
          </div>

          <MealPlanCalendar
            plan={plan}
            recipes={recipes}
            weekOffset={offset}
            onPrevWeek={() => setOffset(data?.prevOffset ?? offset - 1)}
            onNextWeek={() => setOffset(data?.nextOffset ?? offset + 1)}
            onAddMeal={handleAddMeal}
            onDeleteMeal={handleDeleteMeal}
            onMoveMeal={() => mutate()}
          />

          {showShareModal && (
            <ShareModal
              sharedWith={(data?.sharedWith ?? []).map((u) => u.userId)}
              onShare={handleShare}
              onUnshare={handleUnshare}
              onClose={() => setShowShareModal(false)}
            />
          )}
        </>
      )}
    </main>
  )
}
