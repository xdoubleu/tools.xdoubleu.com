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
      className={`flex min-w-0 cursor-pointer select-none items-center justify-between gap-1 rounded-lg px-1.5 py-1 ${
        isMoving ? 'bg-accent/20 ring-2 ring-accent' : 'bg-accent/10 hover:bg-accent/20'
      }`}
    >
      <span className="min-w-0 truncate text-sm text-fg">
        {meal.customName || recipe?.name || '?'}
      </span>
      {meal.servings > 1 && <span className="shrink-0 text-xs text-muted">×{meal.servings}</span>}
      {!inMoveMode && (
        <div className="ml-0.5 flex shrink-0 items-center gap-0.5">
          <button
            aria-label="Edit meal"
            onClick={(e) => {
              e.stopPropagation()
              onEditClick(meal)
            }}
            className="flex h-7 w-7 items-center justify-center rounded-md text-sm text-accent hover:bg-accent/20 active:scale-95"
          >
            ✏
          </button>
          <button
            aria-label="Delete meal"
            onClick={(e) => {
              e.stopPropagation()
              onDeleteMeal(meal.id)
            }}
            className="flex h-7 w-7 items-center justify-center rounded-md text-base font-bold text-danger hover:bg-danger/10 active:scale-95"
          >
            ×
          </button>
        </div>
      )}
    </div>
  )
}
