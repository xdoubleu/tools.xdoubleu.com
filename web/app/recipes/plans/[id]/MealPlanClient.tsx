'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import {
  useMealPlan,
  useRecipes,
  useShoppingList,
  useAddMeal,
  useDeleteMeal,
  useSharePlan,
  useUnsharePlan,
  useDeletePlan
} from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/recipes/MealPlanCalendar'
import ShoppingList from '@/components/recipes/ShoppingList'
import type { ShoppingItem as ShoppingItemExport } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem } from '@/lib/gen/recipes/v1/mealplans_pb'
import {
  AddMealRequest,
  DeleteMealRequest,
  SharePlanRequest,
  UnsharePlanRequest,
  DeletePlanRequest
} from '@/lib/gen/recipes/v1/mealplans_pb'

function toExportItem(item: ShoppingItem): ShoppingItemExport {
  return {
    amount: item.amount.toString(),
    unit: item.unit,
    name: item.name
  }
}

export default function MealPlanClient({ id }: { id: string }) {
  const [offset, setOffset] = useState(0)
  const [showShopping, setShowShopping] = useState(false)
  const { data, error, isLoading, mutate } = useMealPlan(id, offset)
  const { data: recipesData } = useRecipes()
  const { data: shoppingData, isLoading: shoppingLoading } = useShoppingList(
    showShopping && data?.plan ? data.plan.id : ''
  )
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
    await addMeal(
      new AddMealRequest({
        planId: plan.id,
        mealDate: date,
        mealSlot: slot,
        recipeId,
        customName,
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

  const handleShare = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!plan || !shareInput.trim()) return
    setShareError(null)
    try {
      await sharePlan(
        new SharePlanRequest({ planId: plan.id, contactUserId: shareInput.trim(), canEdit: false })
      )
      setShareInput('')
      await mutate()
    } catch (err) {
      setShareError(err instanceof Error ? err.message : 'Failed to share plan.')
    }
  }

  const handleUnshare = async (userId: string) => {
    if (!plan) return
    await unsharePlan(new UnsharePlanRequest({ planId: plan.id, targetUserId: userId }))
    await mutate()
  }

  const handleDelete = async () => {
    if (!plan) return
    await deletePlan(new DeletePlanRequest({ id: plan.id }))
    router.push('/recipes/plans')
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
              <button
                onClick={() => setShowShopping((v) => !v)}
                className="px-4 py-2 bg-surface border border-border rounded hover:bg-border text-sm"
              >
                {showShopping ? 'Hide Shopping List' : 'Shopping List'}
              </button>
              {isOwner && (
                <>
                  <Link
                    href={`/recipes/plans/${plan.id}/edit`}
                    className="px-4 py-2 bg-surface border border-border rounded hover:bg-border text-sm"
                  >
                    Edit
                  </Link>
                  {deleteConfirm ? (
                    <div className="flex gap-2 items-center">
                      <span className="text-sm text-red-600">Delete this plan?</span>
                      <button
                        onClick={handleDelete}
                        className="px-3 py-1 bg-red-600 text-white rounded hover:bg-red-700 text-sm"
                      >
                        Yes, delete
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(false)}
                        className="px-3 py-1 bg-surface border border-border rounded text-sm"
                      >
                        Cancel
                      </button>
                    </div>
                  ) : (
                    <button
                      onClick={() => setDeleteConfirm(true)}
                      className="px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700 text-sm"
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

          {showShopping && (
            <div className="mt-6 border border-border rounded p-4">
              <h2 className="text-lg font-semibold mb-3">Shopping List</h2>
              {shoppingLoading && <p>Loading...</p>}
              {!shoppingLoading && (
                <ShoppingList items={(shoppingData?.items ?? []).map(toExportItem)} />
              )}
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
                  className="flex-1 px-3 py-2 border border-input-border bg-input text-input-text rounded text-sm"
                />
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
                >
                  Share
                </button>
              </form>
              {shareError && <p className="text-sm text-red-600 mb-2">{shareError}</p>}
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
                    className="flex-1 px-3 py-2 border border-input-border bg-input text-input-text rounded text-sm"
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
