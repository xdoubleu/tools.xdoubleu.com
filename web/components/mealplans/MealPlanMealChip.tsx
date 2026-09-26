'use client'

import { useEffect, useState } from 'react'
import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { Button } from '@/components/ui/button'
import { MenuItem } from '@/components/ui/menu-item'
import { Popover } from '@/components/ui/popover'
import { cn } from '@/lib/cn'
import { parseCustomItems, formatCustomItemLabel } from '@/lib/customItems'

interface MealPlanMealChipProps {
  meal: PlanMeal
  recipe: Recipe | undefined
  isSwapping: boolean
  inSwapMode: boolean
  onMealClick: (meal: PlanMeal) => void
  onSwapClick: (meal: PlanMeal) => void
  onEditClick: (meal: PlanMeal) => void
  onDeleteMeal: (mealId: string) => void
}

export default function MealPlanMealChip({
  meal,
  recipe,
  isSwapping,
  inSwapMode,
  onMealClick,
  onSwapClick,
  onEditClick,
  onDeleteMeal
}: MealPlanMealChipProps) {
  const excluded = meal.excludeFromShoppingList
  const customItems = meal.customName ? parseCustomItems(meal.customName) : []
  const isCustom = customItems.length > 0
  const [expanded, setExpanded] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)

  useEffect(() => {
    if (inSwapMode) {
      setExpanded(false)
      setMenuOpen(false)
    }
  }, [inSwapMode])

  const handleBodyClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (inSwapMode) {
      onMealClick(meal)
      return
    }
    setExpanded((v) => !v)
  }

  const runAction = (action: () => void) => (e: React.MouseEvent) => {
    e.stopPropagation()
    setMenuOpen(false)
    action()
  }

  const clamp = !expanded && 'line-clamp-2'

  return (
    <div
      onClick={handleBodyClick}
      aria-expanded={!inSwapMode ? expanded : undefined}
      className={cn(
        'flex min-w-0 cursor-pointer select-none items-start justify-between gap-1 rounded-xl px-1.5 py-1',
        isSwapping
          ? 'bg-accent/20 ring-2 ring-accent'
          : excluded
            ? 'bg-surface hover:bg-hover active:bg-hover'
            : 'bg-accent/10 hover:bg-accent/20 active:bg-accent/20'
      )}
    >
      <div className="min-w-0 flex-1">
        {isCustom ? (
          <ul className={cn('space-y-0.5', clamp)}>
            {customItems.map((item, i) => (
              <li
                key={i}
                className={cn('wrap-break-word text-xs', excluded ? 'text-muted' : 'text-fg')}
              >
                • {formatCustomItemLabel(item)}
              </li>
            ))}
          </ul>
        ) : (
          <span className={cn('wrap-break-word text-sm text-fg', clamp)}>
            {recipe?.name || '?'}
          </span>
        )}
      </div>
      {!isCustom && meal.servings > 1 && (
        <span className="shrink-0 pt-0.5 text-xs text-muted">×{meal.servings}</span>
      )}
      {inSwapMode ? (
        // Reserve the trigger's width so chips keep a constant size in swap mode.
        <span aria-hidden className="ml-0.5 h-6 w-6 shrink-0" />
      ) : (
        <div className="ml-0.5 shrink-0">
          <Popover
            open={menuOpen}
            onOpenChange={setMenuOpen}
            className="w-32 min-w-32 p-1"
            trigger={({ open, onClick }) => (
              <Button
                variant="ghost"
                size="iconSm"
                aria-label="Meal actions"
                aria-haspopup="menu"
                aria-expanded={open}
                onClick={(e) => {
                  e.stopPropagation()
                  onClick()
                }}
                className="text-muted hover:bg-accent/20 hover:text-fg focus-visible:ring-accent"
              >
                ⋯
              </Button>
            )}
          >
            <div role="menu">
              <MenuItem role="menuitem" onClick={runAction(() => onSwapClick(meal))}>
                Swap
              </MenuItem>
              <MenuItem role="menuitem" onClick={runAction(() => onEditClick(meal))}>
                Edit
              </MenuItem>
              <MenuItem
                role="menuitem"
                onClick={runAction(() => onDeleteMeal(meal.id))}
                className="text-danger hover:bg-danger/10"
              >
                Delete
              </MenuItem>
            </div>
          </Popover>
        </div>
      )}
    </div>
  )
}
