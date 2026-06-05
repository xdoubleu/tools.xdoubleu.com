'use client'

import { useEffect, useRef, useState } from 'react'
import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { Button } from '@/components/ui/button'
import { MenuItem } from '@/components/ui/menu-item'

interface MealPlanMealChipProps {
  meal: PlanMeal
  recipe: Recipe | undefined
  isMoving: boolean
  inMoveMode: boolean
  onMealClick: (meal: PlanMeal) => void
  onMoveClick: (meal: PlanMeal) => void
  onEditClick: (meal: PlanMeal) => void
  onDeleteMeal: (mealId: string) => void
}

export default function MealPlanMealChip({
  meal,
  recipe,
  isMoving,
  inMoveMode,
  onMealClick,
  onMoveClick,
  onEditClick,
  onDeleteMeal
}: MealPlanMealChipProps) {
  const customItems = meal.customName ? meal.customName.split('\n').filter(Boolean) : []
  const isCustom = customItems.length > 0
  const fullText = isCustom ? customItems.join('\n') : recipe?.name || '?'

  const [expanded, setExpanded] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!menuOpen) return
    const onPointerDown = (e: MouseEvent) => {
      if (e.target instanceof Node && menuRef.current && !menuRef.current.contains(e.target)) {
        setMenuOpen(false)
      }
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [menuOpen])

  // Collapse any expansion / close the menu whenever we leave the chip's normal state.
  useEffect(() => {
    if (inMoveMode) {
      setExpanded(false)
      setMenuOpen(false)
    }
  }, [inMoveMode])

  const handleBodyClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (inMoveMode) {
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

  const clamp = expanded ? '' : 'line-clamp-2'

  return (
    <div
      onClick={handleBodyClick}
      title={fullText}
      aria-expanded={!inMoveMode ? expanded : undefined}
      className={`flex min-w-0 cursor-pointer select-none items-start justify-between gap-1 rounded-xl px-1.5 py-1 ${
        isMoving ? 'bg-accent/20 ring-2 ring-accent' : 'bg-accent/10 hover:bg-accent/20'
      }`}
    >
      <div className="min-w-0 flex-1">
        {isCustom ? (
          <ul className={`space-y-0.5 ${clamp}`}>
            {customItems.map((item, i) => (
              <li key={i} className="wrap-break-word text-xs text-fg">
                • {item}
              </li>
            ))}
          </ul>
        ) : (
          <span className={`wrap-break-word text-sm text-fg ${clamp}`}>{recipe?.name || '?'}</span>
        )}
      </div>
      {!isCustom && meal.servings > 1 && (
        <span className="shrink-0 pt-0.5 text-xs text-muted">×{meal.servings}</span>
      )}
      {inMoveMode ? (
        // Reserve the trigger's width so chips keep a constant size in move mode.
        <span aria-hidden className="ml-0.5 h-6 w-6 shrink-0" />
      ) : (
        <div ref={menuRef} className="relative ml-0.5 shrink-0">
          <Button
            variant="ghost"
            size="iconSm"
            aria-label="Meal actions"
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            onClick={(e) => {
              e.stopPropagation()
              setMenuOpen((v) => !v)
            }}
            className="text-muted hover:bg-accent/20 hover:text-fg focus-visible:ring-accent"
          >
            ⋯
          </Button>
          {menuOpen && (
            <div
              role="menu"
              className="absolute right-0 z-10 mt-1 w-32 rounded-2xl border border-border bg-card p-1 shadow-elevated"
            >
              <MenuItem role="menuitem" onClick={runAction(() => onMoveClick(meal))}>
                <span aria-hidden>↪</span> Move
              </MenuItem>
              <MenuItem role="menuitem" onClick={runAction(() => onEditClick(meal))}>
                <span aria-hidden>✏</span> Edit
              </MenuItem>
              <MenuItem
                role="menuitem"
                onClick={runAction(() => onDeleteMeal(meal.id))}
                className="text-danger hover:bg-danger/10"
              >
                <span aria-hidden>🗑</span> Delete
              </MenuItem>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
