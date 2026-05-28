'use client'

import { useState } from 'react'
import { useCustomList } from '@/hooks/useShoppingList'
import ShoppingList from '@/components/recipes/ShoppingList'
import ExportModal from '@/components/recipes/ExportModal'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
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
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState('')
  const [newUnit, setNewUnit] = useState('')
  const [adding, setAdding] = useState(false)
  const [showExport, setShowExport] = useState(false)

  const { data, isLoading, mutate } = useCustomList()
  const items = (data?.items ?? []).map(toExportItem)

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    setAdding(true)
    try {
      const client = createServiceClient(ShoppingListService)
      await client.addShoppingItem({
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
    await client.deleteShoppingItem({ itemId })
    await mutate()
  }

  return (
    <main className="max-w-3xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">Shopping List</h1>

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

      {isLoading && <p>Loading...</p>}
      {!isLoading && (
        <ShoppingList items={items} onDelete={handleDelete} onExport={() => setShowExport(true)} />
      )}

      {showExport && <ExportModal customItems={items} onClose={() => setShowExport(false)} />}
    </main>
  )
}
