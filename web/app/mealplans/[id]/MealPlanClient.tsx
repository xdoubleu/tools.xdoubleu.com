'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import {
  useMealPlan,
  useAddMeal,
  useDeleteMeal,
  useSharePlan,
  useUnsharePlan,
  useDeletePlan
} from '@/hooks/useMealPlans'
import type {
  AddMealInput,
  DeleteMealInput,
  SharePlanInput,
  UnsharePlanInput,
  DeletePlanInput
} from '@/hooks/useMealPlans'
import { useRecipes } from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/recipes/MealPlanCalendar'

export default function MealPlanClient({ id }: { id: string }) {
  const [offset, setOffset] = useState(0)
  const { data, error, isLoading, mutate } = useMealPlan(id, offset)
  const { data: recipesData } = useRecipes()
  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()
  const sharePlan = useSharePlan()
  const unsharePlan = useUnsharePlan()
  const deletePlan = useDeletePlan()
  const router = useRouter()

  const [shareInput, setShareInput] = useState('')
  const [unshareInput, setUnshareInput] = useState('')
  const [shareError, setShareError] = useState<string | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState(false)

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

  const handleShare = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!plan || !shareInput.trim()) return
    setShareError(null)
    try {
      const req: SharePlanInput = {
        planId: plan.id,
        contactUserId: shareInput.trim(),
        canEdit: false
      }
      await sharePlan(req)
      setShareInput('')
      await mutate()
    } catch (err) {
      setShareError(err instanceof Error ? err.message : 'Failed to share plan.')
    }
  }

  const handleUnshare = async (userId: string) => {
    if (!plan) return
    const req: UnsharePlanInput = { planId: plan.id, targetUserId: userId }
    await unsharePlan(req)
    await mutate()
  }

  const handleDelete = async () => {
    if (!plan) return
    const req: DeletePlanInput = { id: plan.id }
    await deletePlan(req)
    router.push('/mealplans')
  }

  return (
    <main className="max-w-5xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/mealplans" className="text-sm text-accent hover:underline">
          &larr; Meal Plans
        </Link>
      </div>

      {isLoading && <p>Loading meal plan...</p>}
      {error && <p className="text-danger">Failed to load meal plan.</p>}

      {plan && (
        <>
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-3xl font-bold">{plan.name}</h1>
            <div className="flex gap-2">
              {isOwner && (
                <>
                  <Link
                    href={`/mealplans/${plan.id}/edit`}
                    className="px-4 py-2 bg-surface border border-border rounded hover:bg-border text-sm"
                  >
                    Edit
                  </Link>
                  {deleteConfirm ? (
                    <div className="flex gap-2 items-center">
                      <span className="text-sm text-danger">Delete this plan?</span>
                      <button
                        onClick={handleDelete}
                        className="rounded-xl bg-danger px-3 py-1 text-sm text-white hover:opacity-90"
                      >
                        Yes, delete
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(false)}
                        className="rounded-xl border border-border bg-surface px-3 py-1 text-sm"
                      >
                        Cancel
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setDeleteConfirm(true)}
                      className="rounded-xl bg-danger px-4 py-2 text-sm text-white hover:opacity-90"
                    >
                      Delete
                    </button>
                  )}
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

          {data?.icalUrl && (
            <div className="mt-6 p-4 border border-border rounded">
              <p className="text-sm font-medium text-subtle mb-1">iCal Feed URL</p>
              <p className="text-xs text-muted break-all">{data.icalUrl}</p>
            </div>
          )}

          {isOwner && (
            <div className="mt-6 border border-border rounded p-4">
              <h2 className="text-lg font-semibold mb-3">Sharing</h2>
              <form onSubmit={handleShare} className="flex gap-2 mb-3">
                <input
                  type="text"
                  value={shareInput}
                  onChange={(e) => setShareInput(e.target.value)}
                  placeholder="Contact user ID"
                  className="h-11 flex-1 rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
                />
                <button
                  type="submit"
                  className="rounded-xl bg-accent px-4 py-2 text-sm text-white hover:bg-accent-hover"
                >
                  Share
                </button>
              </form>
              {shareError && <p className="mb-2 text-sm text-danger">{shareError}</p>}
              <div className="mt-3">
                <label className="block text-sm font-medium text-subtle mb-1">
                  Unshare with user ID
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={unshareInput}
                    onChange={(e) => setUnshareInput(e.target.value)}
                    placeholder="User ID to unshare"
                    className="h-11 flex-1 rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
                  />
                  <button
                    type="button"
                    onClick={() => {
                      if (unshareInput) {
                        handleUnshare(unshareInput)
                        setUnshareInput('')
                      }
                    }}
                    className="px-4 py-2 bg-surface border border-border rounded hover:bg-border text-sm"
                  >
                    Unshare
                  </button>
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </main>
  )
}
