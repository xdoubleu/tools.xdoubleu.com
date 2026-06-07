'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { Button } from '@/components/ui/button'
import { MenuItem } from '@/components/ui/menu-item'
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
  const fullText = isCustom
    ? customItems.map(formatCustomItemLabel).join('\n')
    : recipe?.name || '?'

  const [expanded, setExpanded] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const [openUp, setOpenUp] = useState(false)
  const [menuStyle, setMenuStyle] = useState<React.CSSProperties>({})
  const menuRef = useRef<HTMLDivElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  // Approximate height of the 3-item menu; used to decide flip direction.
  const MENU_HEIGHT = 140

  // The menu is rendered in a portal with `position: fixed` so it never
  // contributes to the document's scroll height — opening it on a chip at the
  // bottom of the list can no longer grow the page and force a sudden scroll.
  // We still flip it upward when there isn't enough room below in the viewport.
  const computePosition = useCallback(() => {
    const el = menuRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const spaceBelow = window.innerHeight - rect.bottom
    const up = spaceBelow < MENU_HEIGHT && rect.top > spaceBelow
    setOpenUp(up)
    setMenuStyle({
      position: 'fixed',
      right: Math.max(8, window.innerWidth - rect.right),
      ...(up ? { bottom: window.innerHeight - rect.top + 4 } : { top: rect.bottom + 4 })
    })
  }, [])

  const toggleMenu = () => {
    if (!menuOpen) computePosition()
    setMenuOpen((open) => !open)
  }

  useEffect(() => {
    if (!menuOpen) return
    const reposition = () => computePosition()
    const onPointerDown = (e: MouseEvent) => {
      if (
        e.target instanceof Node &&
        !menuRef.current?.contains(e.target) &&
        !panelRef.current?.contains(e.target)
      ) {
        setMenuOpen(false)
      }
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setMenuOpen(false)
    }
    window.addEventListener('scroll', reposition, true)
    window.addEventListener('resize', reposition)
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('scroll', reposition, true)
      window.removeEventListener('resize', reposition)
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [menuOpen, computePosition])

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
        isSwapping
          ? 'bg-accent/20 ring-2 ring-accent'
          : excluded
            ? 'bg-surface hover:bg-hover'
            : 'bg-accent/10 hover:bg-accent/20'
      }`}
    >
      <div className="min-w-0 flex-1">
        {isCustom ? (
          <ul className={`space-y-0.5 ${clamp}`}>
            {customItems.map((item, i) => (
              <li
                key={i}
                className={`wrap-break-word text-xs ${excluded ? 'text-muted' : 'text-fg'}`}
              >
                • {formatCustomItemLabel(item)}
                {excluded && i === 0 && <span aria-hidden> 🚫</span>}
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
          {menuOpen &&
            createPortal(
              <div
                ref={panelRef}
                role="menu"
                data-open-up={openUp || undefined}
                style={menuStyle}
                className="z-50 w-32 rounded-2xl border border-border bg-card p-1 shadow-elevated"
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
              </div>,
              document.body
            )}
        </div>
      )}
    </div>
  )
}
