'use client'

import React, { useState, useEffect } from 'react'
import { getWeekDates, formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'
import { useAddMeal, useDeleteMeal, useMoveMeal } from '@/hooks/useMealPlans'
import type { AddMealInput, DeleteMealInput, MoveMealInput } from '@/hooks/useMealPlans'
import type { Plan, PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import MealPlanEntryForm from './MealPlanEntryForm'
import MealPlanMealChip from './MealPlanMealChip'
import { Button } from '@/components/ui/button'

interface MealPlanCalendarProps {
  plan: Plan
  recipes: Recipe[]
  weekOffset: number
  onPrevWeek: () => void
  onNextWeek: () => void
  onAddMeal: (
    date: string,
    slot: string,
    recipeId: string,
    customName: string,
    servings: number
  ) => void
  onDeleteMeal: (mealId: string) => void
  onMoveMeal?: () => void
}

export default function MealPlanCalendar({
  plan,
  recipes,
  weekOffset,
  onPrevWeek,
  onNextWeek,
  onAddMeal,
  onDeleteMeal,
  onMoveMeal
}: MealPlanCalendarProps) {
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [movingMeal, setMovingMeal] = useState<PlanMeal | null>(null)
  const [editingMeal, setEditingMeal] = useState<PlanMeal | null>(null)

  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()
  const moveMeal = useMoveMeal()

  const weekDates = getWeekDates(weekOffset)
  const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
  const today = formatMealDate(new Date())

  const getMealsForSlot = (date: string, slot: string) =>
    (plan.meals || []).filter((m) => m.mealDate === date && m.mealSlot === slot)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (movingMeal) setMovingMeal(null)
        if (editingMeal) setEditingMeal(null)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [movingMeal, editingMeal])

  const handleSaveAdd = async (recipeId: string, customName: string, servings: number) => {
    if (!selectedSlot || !selectedDate) return
    try {
      const req: AddMealInput = {
        planId: plan.id,
        mealDate: selectedDate,
        mealSlot: selectedSlot,
        recipeId,
        customName,
        servings
      }
      await addMeal(req)
      const date = selectedDate
      const slot = selectedSlot
      setSelectedSlot(null)
      setSelectedDate(null)
      onAddMeal(date, slot, recipeId, customName, servings)
    } catch (err) {
      console.error('Failed to add meal:', err)
    }
  }

  const handleDeleteMeal = async (mealId: string) => {
    try {
      const req: DeleteMealInput = { planId: plan.id, mealId }
      await deleteMeal(req)
      onDeleteMeal(mealId)
    } catch (err) {
      console.error('Failed to delete meal:', err)
    }
  }

  const handleMealClick = (meal: PlanMeal) => {
    if (!movingMeal) {
      setMovingMeal(meal)
      setSelectedSlot(null)
      setSelectedDate(null)
      return
    }
    if (movingMeal.id === meal.id) {
      setMovingMeal(null)
      return
    }
    handlePlaceMove(meal.mealDate, meal.mealSlot)
  }

  const handleCellClick = (date: string, slot: string, hasMeals: boolean) => {
    if (movingMeal) {
      handlePlaceMove(date, slot)
      return
    }
    if (!hasMeals) {
      setSelectedSlot(slot)
      setSelectedDate(date)
    }
  }

  const handlePlaceMove = async (newDate: string, newSlot: string) => {
    if (!movingMeal) return
    try {
      const req: MoveMealInput = { planId: plan.id, mealId: movingMeal.id, newDate, newSlot }
      await moveMeal(req)
      setMovingMeal(null)
      onMoveMeal?.()
    } catch (err) {
      console.error('Failed to move meal:', err)
    }
  }

  const handleEditClick = (meal: PlanMeal) => {
    setMovingMeal(null)
    setSelectedSlot(null)
    setSelectedDate(null)
    setEditingMeal(meal)
  }

  const handleSaveEdit = async (recipeId: string, customName: string, servings: number) => {
    if (!editingMeal) return
    try {
      const req: AddMealInput = {
        planId: plan.id,
        mealDate: editingMeal.mealDate,
        mealSlot: editingMeal.mealSlot,
        recipeId,
        customName,
        servings
      }
      await addMeal(req)
      const date = editingMeal.mealDate
      const slot = editingMeal.mealSlot
      setEditingMeal(null)
      onAddMeal(date, slot, recipeId, customName, servings)
    } catch (err) {
      console.error('Failed to edit meal:', err)
    }
  }

  const movingMealName =
    movingMeal?.customName || recipes.find((r) => r.id === movingMeal?.recipeId)?.name || '?'

  const renderCell = (formattedDate: string, slot: string) => {
    const mealsInSlot = getMealsForSlot(formattedDate, slot)
    return (
      <div
        key={`${formattedDate}-${slot}`}
        className={`min-h-8 rounded-lg border p-1 ${movingMeal ? 'hover:border-accent/50 hover:bg-accent/10' : 'border-border'}`}
        onClick={() => handleCellClick(formattedDate, slot, mealsInSlot.length > 0)}
      >
        {mealsInSlot.length > 0 ? (
          <div className="space-y-1">
            {mealsInSlot.map((meal) => (
              <MealPlanMealChip
                key={meal.id}
                meal={meal}
                recipe={recipes.find((r) => r.id === meal.recipeId)}
                isMoving={movingMeal?.id === meal.id}
                inMoveMode={!!movingMeal}
                onMealClick={handleMealClick}
                onEditClick={handleEditClick}
                onDeleteMeal={handleDeleteMeal}
              />
            ))}
          </div>
        ) : (
          !movingMeal && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                setSelectedSlot(slot)
                setSelectedDate(formattedDate)
              }}
              className="h-full w-full rounded-lg p-1 text-center text-muted hover:bg-surface"
            >
              +
            </button>
          )
        )}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <Button variant="secondary" size="sm" onClick={onPrevWeek}>
          ← Prev
        </Button>
        <span className="text-sm font-semibold text-fg text-center">
          {weekDates[0].toLocaleDateString('en-GB')} – {weekDates[6].toLocaleDateString('en-GB')}
        </span>
        <Button variant="secondary" size="sm" onClick={onNextWeek}>
          Next →
        </Button>
      </div>

      {movingMeal && (
        <div className="flex items-center justify-between rounded-xl border border-accent/30 bg-accent/10 px-3 py-2 text-sm text-accent">
          <span>
            Moving <strong>{movingMealName}</strong> — click a cell to place it
          </span>
          <button
            onClick={() => setMovingMeal(null)}
            className="ml-4 font-medium underline hover:no-underline"
          >
            Cancel
          </button>
        </div>
      )}

      {/* Mobile: stacked by day */}
      <div className={`sm:hidden space-y-3 text-xs${movingMeal ? ' cursor-crosshair' : ''}`}>
        {weekDates.map((date, i) => {
          const formattedDate = formatMealDate(date)
          const isToday = formattedDate === today
          return (
            <div key={formattedDate} className="rounded-lg border border-border p-2">
              <div className={`mb-2 font-semibold text-sm${isToday ? ' text-accent' : ' text-fg'}`}>
                {dayNames[i]} {date.getDate()}
                {isToday && <span className="ml-1 text-xs font-normal">(today)</span>}
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
      <div className={`hidden sm:block overflow-x-auto${movingMeal ? ' cursor-crosshair' : ''}`}>
        <div
          className="grid gap-1 text-xs"
          style={{ gridTemplateColumns: 'minmax(4.5rem, auto) repeat(7, 1fr)' }}
        >
          <div />
          {weekDates.map((date, i) => {
            const isToday = formatMealDate(date) === today
            return (
              <div key={dayNames[i]} className={`py-1 text-center font-semibold${isToday ? ' text-accent' : ' text-fg'}`}>
                {dayNames[i]}
              </div>
            )
          })}

          <div />
          {weekDates.map((date) => {
            const isToday = formatMealDate(date) === today
            return (
              <div key={formatMealDate(date)} className={`py-1 text-center${isToday ? ' text-accent font-semibold' : ' text-muted'}`}>
                {date.getDate()}
              </div>
            )
          })}

          {MEAL_SLOTS.map((slot) => (
            <React.Fragment key={slot}>
              <div className="flex items-center pr-1 text-xs font-medium text-muted">
                {slot.charAt(0).toUpperCase() + slot.slice(1)}
              </div>
              {weekDates.map((date) => renderCell(formatMealDate(date), slot))}
            </React.Fragment>
          ))}
        </div>
      </div>

      {selectedSlot && selectedDate && (
        <MealPlanEntryForm
          title={`Add meal — ${selectedSlot.charAt(0).toUpperCase() + selectedSlot.slice(1)}, ${new Date(selectedDate + 'T00:00:00').toLocaleDateString()}`}
          recipes={recipes}
          saveLabel="Add"
          onSave={handleSaveAdd}
          onCancel={() => {
            setSelectedSlot(null)
            setSelectedDate(null)
          }}
        />
      )}

      {editingMeal && !selectedSlot && (
        <MealPlanEntryForm
          title={`Edit meal — ${editingMeal.mealSlot.charAt(0).toUpperCase() + editingMeal.mealSlot.slice(1)}, ${new Date(editingMeal.mealDate + 'T00:00:00').toLocaleDateString()}`}
          recipes={recipes}
          initialRecipeId={editingMeal.recipeId}
          initialCustomName={editingMeal.customName}
          initialServings={editingMeal.servings}
          onSave={handleSaveEdit}
          onCancel={() => setEditingMeal(null)}
        />
      )}
    </div>
  )
}
