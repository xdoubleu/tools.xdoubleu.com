'use client'

import { useState } from 'react'
import { useMealPlans } from '@/hooks/useMealPlans'
import { useShoppingList } from '@/hooks/useShoppingList'
import ShoppingList from '@/components/recipes/ShoppingList'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_connect'
import type { ShoppingItem as ShoppingItemExport } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

function toExportItem(item: ShoppingItem): ShoppingItemExport {
  return {
    id: item.id || undefined,
    amount: item.amount,
    unit: item.unit,
    name: item.name
  }
}

export default function ShoppingPage() {
  const [selectedPlanId, setSelectedPlanId] = useState('')
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState('')
  const [newUnit, setNewUnit] = useState('')
  const [adding, setAdding] = useState(false)

  const { data: plansData, isLoading: plansLoading } = useMealPlans()
  const { data: shoppingData, isLoading: shoppingLoading, mutate } = useShoppingList(selectedPlanId)

  const items = (shoppingData?.items ?? []).map(toExportItem)

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim() || !selectedPlanId) return
    setAdding(true)
    try {
      const client = createServiceClient(ShoppingListService)
      await client.addShoppingItem({
        planId: selectedPlanId,
        name: newName.trim(),
        amount: newAmount || '0',
        unit: newUnit.trim()
      })
      setNewName('')
      setNewAmount('')
      setNewUnit('')
      await mutate()
    } finally {
      setAdding(false)
    }
  }

  const handleDelete = async (itemId: string) => {
    const client = createServiceClient(ShoppingListService)
    await client.deleteShoppingItem({ planId: selectedPlanId, itemId })
    await mutate()
  }

  return (
    <main className="max-w-3xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
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

      {selectedPlanId && (
        <form onSubmit={handleAdd} className="flex flex-wrap gap-2 mb-6">
          <input
            type="text"
            placeholder="Item name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            required
            className="flex-1 min-w-32 px-3 py-2 border border-input-border bg-input text-input-text rounded text-sm"
          />
          <input
            type="number"
            placeholder="Amount"
            value={newAmount}
            onChange={(e) => setNewAmount(e.target.value)}
            min="0"
            step="any"
            className="w-24 px-3 py-2 border border-input-border bg-input text-input-text rounded text-sm"
          />
          <input
            type="text"
            placeholder="Unit"
            value={newUnit}
            onChange={(e) => setNewUnit(e.target.value)}
            className="w-24 px-3 py-2 border border-input-border bg-input text-input-text rounded text-sm"
          />
          <button
            type="submit"
            disabled={adding || !newName.trim()}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm disabled:opacity-50"
          >
            Add
          </button>
        </form>
      )}

      {selectedPlanId && shoppingLoading && <p>Loading shopping list...</p>}
      {selectedPlanId && !shoppingLoading && <ShoppingList items={items} onDelete={handleDelete} />}
      {!selectedPlanId && (
        <p className="text-muted">Select a meal plan to generate a shopping list.</p>
      )}
    </main>
  )
}
