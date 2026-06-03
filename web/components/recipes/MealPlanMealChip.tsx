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
  const customItems = meal.customName ? meal.customName.split('\n').filter(Boolean) : []
  const isCustom = customItems.length > 0

  return (
    <div
      onClick={(e) => {
        e.stopPropagation()
        onMealClick(meal)
      }}
      className={`flex min-w-0 cursor-pointer select-none items-start justify-between gap-1 rounded-xl px-1.5 py-1 ${
        isMoving ? 'bg-accent/20 ring-2 ring-accent' : 'bg-accent/10 hover:bg-accent/20'
      }`}
    >
      <div className="min-w-0 flex-1">
        {isCustom ? (
          <ul className="space-y-0.5">
            {customItems.map((item, i) => (
              <li key={i} className="wrap-break-word text-xs text-fg">
                • {item}
              </li>
            ))}
          </ul>
        ) : (
          <span className="wrap-break-word text-sm text-fg">{recipe?.name || '?'}</span>
        )}
      </div>
      {!isCustom && meal.servings > 1 && (
        <span className="shrink-0 text-xs text-muted">×{meal.servings}</span>
      )}
      {!inMoveMode && (
        <div className="ml-0.5 flex shrink-0 items-center gap-0.5">
          <button
            aria-label="Edit meal"
            onClick={(e) => {
              e.stopPropagation()
              onEditClick(meal)
            }}
            className="flex h-5 w-5 items-center justify-center rounded text-sm text-accent hover:bg-accent/20 active:scale-95"
          >
            ✏
          </button>
          <button
            aria-label="Delete meal"
            onClick={(e) => {
              e.stopPropagation()
              onDeleteMeal(meal.id)
            }}
            className="flex h-5 w-5 items-center justify-center rounded text-base font-bold text-danger hover:bg-danger/10 active:scale-95"
          >
            ×
          </button>
        </div>
      )}
    </div>
  )
}
