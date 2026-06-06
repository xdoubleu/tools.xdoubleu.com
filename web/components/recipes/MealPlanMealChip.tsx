'use client'

import { useEffect, useRef, useState } from 'react'
import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { Button } from '@/components/ui/button'
import { MenuItem } from '@/components/ui/menu-item'

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
  const customItems = meal.customName ? meal.customName.split('\n').filter(Boolean) : []
  const isCustom = customItems.length > 0
  const fullText = isCustom ? customItems.join('\n') : recipe?.name || '?'

  const [expanded, setExpanded] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const [openUp, setOpenUp] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  // Approximate height of the 3-item menu; used to decide flip direction.
  const MENU_HEIGHT = 140

  // Open upward when there isn't enough room below the trigger, so a chip near
  // the bottom of the viewport doesn't push the page down and force a scroll.
  const toggleMenu = () => {
    setMenuOpen((open) => {
      if (!open && menuRef.current) {
        const rect = menuRef.current.getBoundingClientRect()
        const spaceBelow = window.innerHeight - rect.bottom
        setOpenUp(spaceBelow < MENU_HEIGHT && rect.top > spaceBelow)
      }
      return !open
    })
  }

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

  const clamp = expanded ? '' : 'line-clamp-2'

  return (
    <div
      onClick={handleBodyClick}
      title={fullText}
      aria-expanded={!inSwapMode ? expanded : undefined}
      className={`flex min-w-0 cursor-pointer select-none items-start justify-between gap-1 rounded-xl px-1.5 py-1 ${
        isSwapping ? 'bg-accent/20 ring-2 ring-accent' : 'bg-accent/10 hover:bg-accent/20'
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
      {inSwapMode ? (
        // Reserve the trigger's width so chips keep a constant size in swap mode.
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
              toggleMenu()
            }}
            className="text-muted hover:bg-accent/20 hover:text-fg focus-visible:ring-accent"
          >
            ⋯
          </Button>
          {menuOpen && (
            <div
              role="menu"
              className={`absolute right-0 z-10 w-32 rounded-2xl border border-border bg-card p-1 shadow-elevated ${
                openUp ? 'bottom-full mb-1' : 'top-full mt-1'
              }`}
            >
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
          )}
        </div>
      )}
    </div>
  )
}
