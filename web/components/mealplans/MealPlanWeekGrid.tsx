'use client'

import React from 'react'
import { formatMealDate, MEAL_SLOTS } from '@/lib/mealplans/mealPlanCalendar'
import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import MealPlanMealChip from './MealPlanMealChip'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/cn'

const DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

interface MealPlanWeekGridProps {
  weekDates: Date[]
  recipes: Recipe[]
  swappingMeal: PlanMeal | null
  getMealsForSlot: (date: string, slot: string) => PlanMeal[]
  onCellClick: (date: string, slot: string) => void
  onMealClick: (meal: PlanMeal) => void
  onStartSwap: (meal: PlanMeal) => void
  onEditClick: (meal: PlanMeal) => void
  onDeleteMeal: (mealId: string) => void
  onAddClick: (date: string, slot: string) => void
  onFillDay: (date: string) => void
}

// Stacked by day on mobile, a 7-column grid on desktop; state lives in the parent.
export default function MealPlanWeekGrid({
  weekDates,
  recipes,
  swappingMeal,
  getMealsForSlot,
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
    const meals = getMealsForSlot(formattedDate, slot)
    const hasMeals = meals.length > 0
    return (
      <Card
        key={`${formattedDate}-${slot}`}
        className={cn(
          'min-h-14 min-w-0 space-y-1 rounded-xl p-1.5 shadow-none',
          swappingMeal && 'border-dashed border-accent/50 bg-accent/5 hover:bg-accent/10'
        )}
        onClick={() => onCellClick(formattedDate, slot)}
      >
        {meals.map((meal) => (
          <MealPlanMealChip
            key={meal.id}
            meal={meal}
            recipe={recipes.find((r) => r.id === meal.recipeId)}
            isSwapping={swappingMeal?.id === meal.id}
            inSwapMode={!!swappingMeal}
            onMealClick={onMealClick}
            onSwapClick={onStartSwap}
            onEditClick={onEditClick}
            onDeleteMeal={onDeleteMeal}
          />
        ))}
        {swappingMeal ? (
          !hasMeals && (
            <div className="flex h-full min-h-10 items-center justify-center text-xs text-muted">
              Place here
            </div>
          )
        ) : (
          <Button
            variant="ghost"
            aria-label={hasMeals ? 'Add another meal' : 'Add meal'}
            onClick={(e) => {
              e.stopPropagation()
              onAddClick(formattedDate, slot)
            }}
            className={cn(
              'min-h-11 w-full px-0 text-muted',
              hasMeals ? 'text-base' : 'h-full text-lg'
            )}
          >
            +
          </Button>
        )}
      </Card>
    )
  }

  return (
    <div className="flex flex-col gap-4 items-start">
      <div className="w-full min-w-0">
        <div className={cn('space-y-3 text-xs sm:hidden', swappingMeal && 'cursor-crosshair')}>
          {weekDates.map((date) => {
            const formattedDate = formatMealDate(date)
            const isToday = formattedDate === today
            return (
              <Card key={formattedDate} className="rounded-xl p-2 shadow-none">
                <div className="mb-2 flex items-center justify-between gap-2">
                  <div className={cn('text-sm font-semibold', isToday ? 'text-accent' : 'text-fg')}>
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
              </Card>
            )
          })}
        </div>

        <div className={cn('hidden overflow-x-auto sm:block', swappingMeal && 'cursor-crosshair')}>
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
                  className={cn(
                    'flex flex-col items-center gap-0.5 py-1 text-center font-semibold',
                    isToday ? 'text-accent' : 'text-fg'
                  )}
                >
                  {DAY_NAMES[date.getDay()]}
                  <Button
                    variant="ghost"
                    size="sm"
                    className="rounded-lg px-1.5 text-xs font-normal text-muted"
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
                  className={cn(
                    'py-1 text-center',
                    isToday ? 'font-semibold text-accent' : 'text-muted'
                  )}
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
