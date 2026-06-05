'use client'

import { useState } from 'react'
import { useItemNames, useCategories } from '@/hooks/useShoppingList'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { Select } from '@/components/ui/select'

export default function ItemCatalog() {
  const { data: namesData, isLoading, mutate } = useItemNames()
  const { data: categoriesData } = useCategories()
  const [error, setError] = useState('')

  const client = createServiceClient(ShoppingListService)
  const names = namesData?.names ?? []
  const categories = categoriesData?.categories ?? []

  const handleChange = async (name: string, categoryId: string) => {
    setError('')
    try {
      await client.setItemCategory({ name, categoryId })
      await mutate()
    } catch {
      setError('Failed to update category.')
    }
  }

  if (isLoading) return <p className="text-sm text-muted">Loading…</p>
  if (names.length === 0) {
    return (
      <p className="text-sm text-muted">
        No items yet. Add custom items or recipe ingredients first.
      </p>
    )
  }

  return (
    <div className="space-y-3">
      {error && <p className="text-sm text-danger">{error}</p>}
      <ul className="space-y-2">
        {names.map((item) => (
          <li
            key={item.name}
            className="flex items-center gap-2 rounded-2xl border border-border bg-surface p-2"
          >
            <span className="flex-1 text-sm text-fg">
              {item.name}
              {!item.categoryId && (
                <span className="ml-2 text-xs font-medium text-danger">unassigned</span>
              )}
            </span>
            <Select
              aria-label={`Category for ${item.name}`}
              value={item.categoryId}
              onChange={(e) => handleChange(item.name, e.target.value)}
              className="h-9 w-auto px-2"
            >
              <option value="">-- Unassigned --</option>
              {categories.map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </Select>
          </li>
        ))}
      </ul>
    </div>
  )
}
