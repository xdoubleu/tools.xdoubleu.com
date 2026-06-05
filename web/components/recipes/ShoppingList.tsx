'use client'

import { useState } from 'react'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'
import { Button } from '@/components/ui/button'

interface ShoppingListProps {
  items: ShoppingItem[]
  onDelete?: (itemId: string) => Promise<void>
  onExport?: () => void
}

export default function ShoppingList({ items, onDelete, onExport }: ShoppingListProps) {
  const [checkedItems, setCheckedItems] = useState<Set<string>>(new Set())

  const toggleItem = (key: string) => {
    const newChecked = new Set(checkedItems)
    if (newChecked.has(key)) newChecked.delete(key)
    else newChecked.add(key)
    setCheckedItems(newChecked)
  }

  return (
    <div className="space-y-4">
      {onExport && (
        <Button variant="secondary" size="sm" onClick={onExport}>
          Export
        </Button>
      )}

      {items.length === 0 ? (
        <p className="text-muted">No items yet. Add something above.</p>
      ) : (
        <div className="space-y-2">
          {items.map((item, idx) => {
            const key = `item-${idx}-${item.name}`
            const isChecked = checkedItems.has(key)
            return (
              <div
                key={key}
                className="flex items-center gap-3 rounded-2xl border border-border bg-card p-3"
              >
                <input
                  type="checkbox"
                  checked={isChecked}
                  onChange={() => toggleItem(key)}
                  className="h-4 w-4 rounded accent-[rgb(var(--color-accent))]"
                />
                <span
                  className={`flex-1 text-sm ${isChecked ? 'line-through text-muted' : 'text-fg'}`}
                >
                  {item.amount} {item.unit} - {item.name}
                </span>
                {item.id && onDelete && (
                  <Button
                    variant="ghost"
                    size="iconSm"
                    onClick={() => onDelete(item.id!)}
                    aria-label={`Remove ${item.name}`}
                    className="rounded-full text-muted hover:bg-transparent hover:text-danger focus-visible:ring-danger/50"
                  >
                    ×
                  </Button>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
