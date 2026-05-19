'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useMealPlans, useShoppingList } from '@/hooks/useRecipes'
import ShoppingList from '@/components/recipes/ShoppingList'
import type { ShoppingItem as ShoppingItemExport } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem } from '@/lib/gen/recipes/v1/mealplans_pb'

function toExportItem(item: ShoppingItem): ShoppingItemExport {
  return {
    amount: item.amount.toString(),
    unit: item.unit,
    name: item.name
  }
}

export default function ShoppingPage() {
  const [selectedPlanId, setSelectedPlanId] = useState('')
  const { data: plansData, isLoading: plansLoading } = useMealPlans()
  const { data: shoppingData, isLoading: shoppingLoading } = useShoppingList(selectedPlanId)

  const items = (shoppingData?.items ?? []).map(toExportItem)

  return (
    <main className="max-w-3xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/recipes" className="text-blue-600 hover:underline text-sm">
          &larr; Recipes
        </Link>
        <h1 className="text-3xl font-bold">Shopping List</h1>
      </div>

      <div className="mb-6">
        <label htmlFor="plan-select" className="block text-sm font-medium text-subtle mb-1">
          Select Meal Plan
        </label>
        {plansLoading && <p className="text-sm text-muted">Loading plans...</p>}
        {!plansLoading && (
          <select
            id="plan-select"
            value={selectedPlanId}
            onChange={(e) => setSelectedPlanId(e.target.value)}
            className="w-full sm:w-auto px-3 py-2 border border-input-border bg-input text-input-text rounded"
          >
            <option value="">-- Select a plan --</option>
            {(plansData?.plans ?? []).map((plan) => (
              <option key={plan.id} value={plan.id}>
                {plan.name}
              </option>
            ))}
          </select>
        )}
      </div>

      {selectedPlanId && shoppingLoading && <p>Loading shopping list...</p>}
      {selectedPlanId && !shoppingLoading && <ShoppingList items={items} />}
      {!selectedPlanId && (
        <p className="text-muted">Select a meal plan to generate a shopping list.</p>
      )}
    </main>
  )
}
