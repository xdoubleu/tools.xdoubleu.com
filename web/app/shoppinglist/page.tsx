'use client'

import { useState } from 'react'
import { useSWRConfig } from 'swr'
import Link from 'next/link'
import { useCustomList, useCategories } from '@/hooks/useShoppingList'
import ShoppingList from '@/components/recipes/ShoppingList'
import ExportModal from '@/components/recipes/ExportModal'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
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
  const [newCategoryId, setNewCategoryId] = useState('')
  const [adding, setAdding] = useState(false)
  const [showExport, setShowExport] = useState(false)

  const { data, isLoading, mutate } = useCustomList()
  const { data: categoriesData } = useCategories()
  const categories = categoriesData?.categories ?? []
  const { mutate: globalMutate } = useSWRConfig()

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    const name = newName.trim()
    if (!name) return
    setAdding(true)
    try {
      const client = createServiceClient(ShoppingListService)
      await client.addShoppingItem({
        name,
        amount: newAmount || '0',
        unit: newUnit.trim()
      })
      // The category lives in the name->category catalog, not on the item, so
      // assigning it here makes it persist by name across every list and export.
      if (newCategoryId) {
        await client.setItemCategory({ name, categoryId: newCategoryId })
        await globalMutate('/shoppinglist/item-categories')
        await globalMutate('/shoppinglist/item-names')
      }
      setNewName('')
      setNewAmount('')
      setNewUnit('')
      setNewCategoryId('')
      await mutate()
    } finally {
      setAdding(false)
    }
  }

  const items = (data?.items ?? []).map(toExportItem)

  const handleDelete = async (itemId: string) => {
    const client = createServiceClient(ShoppingListService)
    await client.deleteShoppingItem({ itemId })
    await mutate()
  }

  return (
    <main className="max-w-3xl mx-auto p-6">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-3xl font-bold">Shopping List</h1>
        <Link href="/shoppinglist/settings" className="text-sm text-accent hover:underline">
          Settings
        </Link>
      </div>

      <form onSubmit={handleAdd} className="flex flex-wrap gap-2 mb-6">
        <Input
          type="text"
          placeholder="Item name"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          required
          className="min-w-32 flex-1"
        />
        <Input
          type="number"
          placeholder="Amount"
          value={newAmount}
          onChange={(e) => setNewAmount(e.target.value)}
          min="0"
          step="any"
          className="w-24"
        />
        <Input
          type="text"
          placeholder="Unit"
          value={newUnit}
          onChange={(e) => setNewUnit(e.target.value)}
          className="w-24"
        />
        {categories.length > 0 && (
          <Select
            aria-label="Category"
            value={newCategoryId}
            onChange={(e) => setNewCategoryId(e.target.value)}
            className="w-auto"
          >
            <option value="">-- Category --</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </Select>
        )}
        <Button type="submit" disabled={adding || !newName.trim()}>
          Add
        </Button>
      </form>

      {isLoading && <p>Loading...</p>}
      {!isLoading && (
        <ShoppingList items={items} onDelete={handleDelete} onExport={() => setShowExport(true)} />
      )}

      {showExport && <ExportModal customItems={items} onClose={() => setShowExport(false)} />}
    </main>
  )
}
