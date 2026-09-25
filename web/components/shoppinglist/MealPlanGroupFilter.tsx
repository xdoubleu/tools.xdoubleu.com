'use client'

import { Checkbox } from '@/components/ui/checkbox'

interface IngredientGroup {
  recipeName: string
  groupName: string
}

interface MealPlanGroupFilterProps {
  groups: IngredientGroup[]
  excludedGroups: Set<string>
  onToggle: (groupName: string) => void
}

// Include/exclude recipe ingredient groups from the meal-plan items; renders
// nothing when there are none.
export default function MealPlanGroupFilter({
  groups,
  excludedGroups,
  onToggle
}: MealPlanGroupFilterProps) {
  if (groups.length === 0) return null

  return (
    <div className="space-y-1.5">
      <h2 className="text-xs font-semibold uppercase tracking-widest text-muted">
        Exclude ingredient groups
      </h2>
      <div className="space-y-1">
        {groups.map((g) => {
          const key = `${g.recipeName}::${g.groupName}`
          const checked = !excludedGroups.has(g.groupName)
          return (
            <label key={key} className="flex items-center gap-2 text-sm">
              <Checkbox checked={checked} onChange={() => onToggle(g.groupName)} />
              <span className="text-fg">{g.groupName}</span>
              <span className="text-muted">({g.recipeName})</span>
            </label>
          )
        })}
      </div>
    </div>
  )
}
