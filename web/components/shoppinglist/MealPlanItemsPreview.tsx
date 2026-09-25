'use client'

import { prepareForExport, formatOrigins } from '@/lib/shoppinglist/shoppingExport'
import type { ShoppingItem } from '@/lib/shoppinglist/shoppingExport'

interface MealPlanItemsPreviewProps {
  mealItems: ShoppingItem[]
  isLoading?: boolean
}

// Read-only preview of meal-plan items via the shared export pipeline
// (custom items render in the editable list above).
export default function MealPlanItemsPreview({ mealItems, isLoading }: MealPlanItemsPreviewProps) {
  if (isLoading) {
    return <p className="text-sm text-muted">Loading…</p>
  }

  const items = prepareForExport([], mealItems)
  if (items.length === 0) return null

  return (
    <div>
      <h2 className="mb-2 text-xs font-semibold uppercase tracking-widest text-muted">
        From meal plans
      </h2>
      <ul className="space-y-1">
        {items.map((item, i) => (
          <li key={i} className="text-sm text-subtle">
            {item.amount} {item.unit} — {item.name}
            {item.origins && item.origins.length > 0 && (
              <span className="text-muted">{formatOrigins(item.origins)}</span>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}
