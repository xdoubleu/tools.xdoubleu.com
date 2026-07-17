'use client'

import { prepareForExport, formatOrigins } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

interface MealPlanItemsPreviewProps {
  mealItems: ShoppingItem[]
  isLoading?: boolean
}

// Read-only preview of the meal-plan-derived items on the shopping-list landing
// page. Mirrors the ExportModal flat preview: it runs the shared merge/combine/
// unit-upgrade pipeline (custom items are passed empty here — they render in the
// editable list above), so nothing is editable and no merge logic is duplicated.
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
