'use client'

import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

interface MealPlanMealChipProps {
  meal: PlanMeal
  recipe: Recipe | undefined
  isMoving: boolean
  inMoveMode: boolean
  onMealClick: (meal: PlanMeal) => void
  onEditClick: (meal: PlanMeal) => void
  onDeleteMeal: (mealId: string) => void
}

export default function MealPlanMealChip({
  meal,
  recipe,
  isMoving,
  inMoveMode,
  onMealClick,
  onEditClick,
  onDeleteMeal
}: MealPlanMealChipProps) {
  return (
    <div
      onClick={(e) => {
        e.stopPropagation()
        onMealClick(meal)
      }}
      className={`flex min-w-0 cursor-pointer select-none items-center justify-between gap-1 rounded-lg p-1 ${
        isMoving ? 'bg-accent/20 ring-2 ring-accent' : 'bg-accent/10 hover:bg-accent/20'
      }`}
    >
      <span className="truncate text-fg">{meal.customName || recipe?.name || '?'}</span>
      {meal.servings > 1 && <span className="shrink-0 text-muted">×{meal.servings}</span>}
      {!inMoveMode && (
        <>
          <button
            aria-label="Edit meal"
            onClick={(e) => {
              e.stopPropagation()
              onEditClick(meal)
            }}
            className="shrink-0 text-xs text-accent hover:text-accent-hover"
          >
            ✏
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation()
              onDeleteMeal(meal.id)
            }}
            className="shrink-0 font-bold text-danger hover:opacity-80"
          >
            ×
          </button>
        </>
      )}
    </div>
  )
}
