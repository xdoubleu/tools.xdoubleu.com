'use client'

import React from 'react'
import { formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'
import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import MealPlanMealChip from './MealPlanMealChip'
import { Button } from '@/components/ui/button'

const DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

interface MealPlanWeekGridProps {
  weekDates: Date[]
  recipes: Recipe[]
  swappingMeal: PlanMeal | null
  getMealForSlot: (date: string, slot: string) => PlanMeal | undefined
  onCellClick: (date: string, slot: string) => void
  onMealClick: (meal: PlanMeal) => void
  onStartSwap: (meal: PlanMeal) => void
  onEditClick: (meal: PlanMeal) => void
  onDeleteMeal: (mealId: string) => void
  onAddClick: (date: string, slot: string) => void
  onFillDay: (date: string) => void
}

// MealPlanWeekGrid renders the week as a stacked-by-day list on mobile and a
// 7-column grid on desktop. All interaction state lives in the parent.
export default function MealPlanWeekGrid({
  weekDates,
  recipes,
  swappingMeal,
  getMealForSlot,
  onCellClick,
  onMealClick,
  onStartSwap,
  onEditClick,
  onDeleteMeal,
  onAddClick,
  onFillDay
}: MealPlanWeekGridProps) {
  const today = formatMealDate(new Date())

  const renderCell = (formattedDate: string, slot: string) => {
    const meal = getMealForSlot(formattedDate, slot)
    return (
      <div
        key={`${formattedDate}-${slot}`}
        className={`min-h-14 min-w-0 rounded-xl border p-1.5 ${swappingMeal ? 'hover:border-accent/50 hover:bg-accent/10' : 'border-border'}`}
        onClick={() => onCellClick(formattedDate, slot)}
      >
        {meal ? (
          <MealPlanMealChip
            meal={meal}
            recipe={recipes.find((r) => r.id === meal.recipeId)}
            isSwapping={swappingMeal?.id === meal.id}
            inSwapMode={!!swappingMeal}
            onMealClick={onMealClick}
            onSwapClick={onStartSwap}
            onEditClick={onEditClick}
            onDeleteMeal={onDeleteMeal}
          />
        ) : swappingMeal ? (
          <div className="flex h-full min-h-10 items-center justify-center text-xs text-muted">
            Place here
          </div>
        ) : (
          <Button
            variant="ghost"
            onClick={(e) => {
              e.stopPropagation()
              onAddClick(formattedDate, slot)
            }}
            className="h-full min-h-10 w-full px-0 text-lg text-muted"
          >
            +
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-4 items-start">
      <div className="w-full min-w-0">
        {/* Mobile: stacked by day */}
        <div className={`sm:hidden space-y-3 text-xs${swappingMeal ? ' cursor-crosshair' : ''}`}>
          {weekDates.map((date) => {
            const formattedDate = formatMealDate(date)
            const isToday = formattedDate === today
            return (
              <div key={formattedDate} className="rounded-xl border border-border p-2">
                <div className="mb-2 flex items-center justify-between gap-2">
                  <div className={`font-semibold text-sm${isToday ? ' text-accent' : ' text-fg'}`}>
                    {DAY_NAMES[date.getDay()]} {date.getDate()}
                    {isToday && <span className="ml-1 text-xs font-normal">(today)</span>}
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="rounded-lg text-xs text-muted"
                    onClick={() => onFillDay(formattedDate)}
                  >
                    Fill day
                  </Button>
                </div>
                <div className="space-y-1">
                  {MEAL_SLOTS.map((slot) => (
                    <div key={slot} className="flex gap-2">
                      <span className="w-16 shrink-0 pt-1 text-xs font-medium text-muted">
                        {slot.charAt(0).toUpperCase() + slot.slice(1)}
                      </span>
                      <div className="flex-1">{renderCell(formattedDate, slot)}</div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>

        {/* Desktop: 7-column grid */}
        <div
          className={`hidden sm:block overflow-x-auto${swappingMeal ? ' cursor-crosshair' : ''}`}
        >
          <div
            className="grid gap-1.5 text-sm"
            style={{ gridTemplateColumns: 'minmax(5rem, auto) repeat(7, 1fr)' }}
          >
            <div />
            {weekDates.map((date) => {
              const formattedDate = formatMealDate(date)
              const isToday = formattedDate === today
              return (
                <div
                  key={formattedDate}
                  className={`flex flex-col items-center gap-0.5 py-1 text-center font-semibold${isToday ? ' text-accent' : ' text-fg'}`}
                >
                  {DAY_NAMES[date.getDay()]}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-auto rounded-lg px-1.5 py-0 text-xs font-normal text-muted"
                    onClick={() => onFillDay(formattedDate)}
                  >
                    Fill day
                  </Button>
                </div>
              )
            })}

            <div />
            {weekDates.map((date) => {
              const isToday = formatMealDate(date) === today
              return (
                <div
                  key={formatMealDate(date)}
                  className={`py-1 text-center${isToday ? ' text-accent font-semibold' : ' text-muted'}`}
                >
                  {date.getDate()}
                </div>
              )
            })}

            {MEAL_SLOTS.map((slot) => (
              <React.Fragment key={slot}>
                <div className="flex items-center pr-2 text-sm font-medium text-muted">
                  {slot.charAt(0).toUpperCase() + slot.slice(1)}
                </div>
                {weekDates.map((date) => renderCell(formatMealDate(date), slot))}
              </React.Fragment>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
