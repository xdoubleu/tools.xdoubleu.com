import { prepareForExport, formatOrigins } from '@/lib/shoppinglist/shoppingExport'
import type { ShoppingItem } from '@/lib/shoppinglist/shoppingExport'
import { SectionHeading } from '@/components/ui/section-heading'
import { LoadingState } from '@/components/ui/states'

interface MealPlanItemsPreviewProps {
  mealItems: ShoppingItem[]
  isLoading?: boolean
}

// Read-only preview of meal-plan items via the shared export pipeline
// (custom items render in the editable list above).
export default function MealPlanItemsPreview({ mealItems, isLoading }: MealPlanItemsPreviewProps) {
  if (isLoading) {
    return <LoadingState placement="inline" />
  }

  const items = prepareForExport([], mealItems)
  if (items.length === 0) return null

  return (
    <div>
      <SectionHeading level="eyebrow" as="h2" className="mb-2">
        From meal plans
      </SectionHeading>
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
