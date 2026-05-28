'use client'

import { useState } from 'react'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

interface ShoppingListProps {
  items: ShoppingItem[]
  onDelete?: (itemId: string) => Promise<void>
  onExport?: () => void
}

export default function ShoppingList({ items, onDelete, onExport }: ShoppingListProps) {
  const [checkedItems, setCheckedItems] = useState<Set<string>>(new Set())

  const toggleItem = (key: string) => {
    const newChecked = new Set(checkedItems)
    if (newChecked.has(key)) {
      newChecked.delete(key)
    } else {
      newChecked.add(key)
    }
    setCheckedItems(newChecked)
  }

  if (items.length === 0) {
    return (
      <div className="space-y-4">
        <p className="text-muted">No items yet. Add something above.</p>
        {onExport && (
          <button
            onClick={onExport}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
          >
            Export
          </button>
        )}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {onExport && (
        <button
          onClick={onExport}
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
        >
          Export
        </button>
      )}

      <div className="space-y-2">
        {items.map((item, idx) => {
          const key = `item-${idx}-${item.name}`
          const isChecked = checkedItems.has(key)
          return (
            <div key={key} className="flex items-center gap-3 p-3 border border-border rounded">
              <input
                type="checkbox"
                checked={isChecked}
                onChange={() => toggleItem(key)}
                className="w-4 h-4 rounded"
              />
              <span className={`flex-1 ${isChecked ? 'line-through text-muted' : ''}`}>
                {item.amount} {item.unit} - {item.name}
              </span>
              {item.id && onDelete && (
                <button
                  onClick={() => onDelete(item.id!)}
                  aria-label={`Remove ${item.name}`}
                  className="text-muted hover:text-red-600 text-sm px-1"
                >
                  ×
                </button>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
