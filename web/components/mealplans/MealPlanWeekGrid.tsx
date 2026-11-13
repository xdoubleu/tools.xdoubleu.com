import React from 'react'
import { CalendarPlus } from 'lucide-react'
import { formatMealDate, MEAL_SLOTS } from '@/lib/mealplans/mealPlanCalendar'
import type { PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import MealPlanMealChip from './MealPlanMealChip'
import type { PickMode } from './useMealCalendarState'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/cn'
import { formatDate } from '@/lib/dates'

const DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const slotLabel = (slot: string) => slot.charAt(0).toUpperCase() + slot.slice(1)

interface MealPlanWeekGridProps {
  weekDates: Date[]
  recipes: Recipe[]
  swappingMeal: PlanMeal | null
  pickMode: PickMode
  getMealsForSlot: (date: string, slot: string) => PlanMeal[]
  onCellClick: (date: string, slot: string) => void
  onMealClick: (meal: PlanMeal) => void
  onStartMove: (meal: PlanMeal) => void
  onStartSwap: (meal: PlanMeal) => void
  onEditClick: (meal: PlanMeal) => void
  onDeleteMeal: (mealId: string) => void
  onAddClick: (date: string, slot: string) => void
  onFillDay: (date: string) => void
  /** Meal whose chip takes focus next, e.g. after a swap moved it. */
  focusMealId?: string
  onMealFocused?: () => void
}

// Stacked by day on mobile, a 7-column grid on desktop; state lives in the parent.
// A cell keeps its height in pick mode, so picking a meal never moves the page.
export default function MealPlanWeekGrid({
  weekDates,
  recipes,
  swappingMeal,
  pickMode,
  getMealsForSlot,
  onCellClick,
  onMealClick,
  onStartMove,
  onStartSwap,
  onEditClick,
  onDeleteMeal,
  onAddClick,
  onFillDay,
  focusMealId,
  onMealFocused
}: MealPlanWeekGridProps) {
  const today = formatMealDate(new Date())

  const renderCell = (formattedDate: string, slot: string) => {
    const meals = getMealsForSlot(formattedDate, slot)
    const hasMeals = meals.length > 0
    return (
      <Card
        key={`${formattedDate}-${slot}`}
        padding="none"
        className={cn(
          'min-h-12 min-w-0 shadow-none sm:min-h-14',
          swappingMeal && 'border-dashed border-accent/50 bg-accent/5 hover:bg-accent/10'
        )}
        onClick={() => onCellClick(formattedDate, slot)}
      >
        <div className="flex h-full min-h-12 flex-col justify-center gap-1 sm:min-h-14 sm:justify-start sm:p-1.5">
          {meals.map((meal) => (
            <MealPlanMealChip
              key={meal.id}
              meal={meal}
              recipe={recipes.find((r) => r.id === meal.recipeId)}
              isSwapping={swappingMeal?.id === meal.id}
              inSwapMode={!!swappingMeal}
              pickMode={pickMode}
              onMealClick={onMealClick}
              onMoveClick={onStartMove}
              onSwapClick={onStartSwap}
              onEditClick={onEditClick}
              onAddClick={() => onAddClick(formattedDate, slot)}
              onDeleteMeal={onDeleteMeal}
              restoreFocus={meal.id === focusMealId}
              onFocusRestored={onMealFocused}
            />
          ))}
          {!hasMeals &&
            (swappingMeal ? (
              <Button
                variant="ghost"
                aria-label={`Place here: ${slotLabel(slot)}, ${formatDate(formattedDate)}`}
                onClick={(e) => {
                  e.stopPropagation()
                  onCellClick(formattedDate, slot)
                }}
                className="min-h-11 w-full px-0 font-normal text-muted hover:bg-transparent active:bg-transparent sm:flex-1"
              >
                Place here
              </Button>
            ) : (
              <Button
                variant="ghost"
                aria-label="Add meal"
                onClick={(e) => {
                  e.stopPropagation()
                  onAddClick(formattedDate, slot)
                }}
                className="min-h-11 w-full px-0 text-muted sm:flex-1"
              >
                +
              </Button>
            ))}
        </div>
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
              <Card key={formattedDate} padding="sm" className="shadow-none">
                <div className="-mt-1.5 mb-1.5 flex items-center justify-between gap-2">
                  <div className={cn('text-sm font-semibold', isToday ? 'text-accent' : 'text-fg')}>
                    {DAY_NAMES[date.getDay()]} {date.getDate()}
                    {isToday && <span className="ml-1 text-xs font-normal">(today)</span>}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label="Fill day"
                    className="text-muted"
                    onClick={() => onFillDay(formattedDate)}
                  >
                    <CalendarPlus className="h-4 w-4" aria-hidden />
                  </Button>
                </div>
                <div className="space-y-1">
                  {MEAL_SLOTS.map((slot) => (
                    <div key={slot} className="flex gap-2">
                      <span className="w-16 shrink-0 pt-4 text-xs font-medium text-muted">
                        {slotLabel(slot)}
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
                    size="icon"
                    aria-label="Fill day"
                    className="text-muted"
                    onClick={() => onFillDay(formattedDate)}
                  >
                    <CalendarPlus className="h-4 w-4" aria-hidden />
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
                  {slotLabel(slot)}
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
