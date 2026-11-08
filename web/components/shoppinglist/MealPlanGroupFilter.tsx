import { SectionHeading } from '@/components/ui/section-heading'
import { TogglePill } from '@/components/ui/toggle-pill'
import { Toolbar } from '@/components/ui/toolbar'

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
    <section className="space-y-2">
      <SectionHeading level="eyebrow" as="h2">
        Include ingredient groups
      </SectionHeading>
      <Toolbar aria-label="Include ingredient groups" className="mb-0">
        {groups.map((g) => (
          <TogglePill
            key={`${g.recipeName}::${g.groupName}`}
            active={!excludedGroups.has(g.groupName)}
            onClick={() => onToggle(g.groupName)}
            label={`${g.groupName} (${g.recipeName})`}
          />
        ))}
      </Toolbar>
    </section>
  )
}
