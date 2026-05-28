'use client'

import React, { useState, useEffect } from 'react'
import { getWeekDates, formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'
import { useAddMeal, useDeleteMeal, useMoveMeal } from '@/hooks/useMealPlans'
import type { AddMealInput, DeleteMealInput, MoveMealInput } from '@/hooks/useMealPlans'
import type { Plan, PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import RecipeCombobox from './RecipeCombobox'
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
  const [selectedRecipeId, setSelectedRecipeId] = useState('')
  const [selectedCustomName, setSelectedCustomName] = useState('')
  const [selectedServings, setSelectedServings] = useState(1)
  const [movingMeal, setMovingMeal] = useState<PlanMeal | null>(null)
  const [editingMeal, setEditingMeal] = useState<PlanMeal | null>(null)
  const [editRecipeId, setEditRecipeId] = useState('')
  const [editCustomName, setEditCustomName] = useState('')
  const [editServings, setEditServings] = useState(1)

  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()
  const moveMeal = useMoveMeal()

  const weekDates = getWeekDates(weekOffset)
  const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

  const getMealsForSlot = (date: string, slot: string) =>
    (plan.meals || []).filter((m) => m.mealDate === date && m.mealSlot === slot)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (movingMeal) setMovingMeal(null)
        if (editingMeal) cancelEdit()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [movingMeal, editingMeal])

  const handleComboboxSelect = (recipeId: string, customName: string) => {
    setSelectedRecipeId(recipeId)
    setSelectedCustomName(customName)
  }

  const handleAddMeal = async () => {
    if (!selectedSlot || !selectedDate) return
    if (!selectedRecipeId && !selectedCustomName.trim()) return
    try {
      const req: AddMealInput = {
        planId: plan.id,
        mealDate: selectedDate,
        mealSlot: selectedSlot,
        recipeId: selectedRecipeId,
        customName: selectedCustomName,
        servings: selectedServings
      }
      await addMeal(req)
      const date = selectedDate
      const slot = selectedSlot
      setSelectedSlot(null)
      setSelectedDate(null)
      setSelectedRecipeId('')
      setSelectedCustomName('')
      setSelectedServings(1)
      onAddMeal(date, slot, selectedRecipeId, selectedCustomName, selectedServings)
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

  const cancelAdd = () => {
    setSelectedSlot(null)
    setSelectedDate(null)
    setSelectedRecipeId('')
    setSelectedCustomName('')
  }

  const handleEditClick = (meal: PlanMeal) => {
    setMovingMeal(null)
    setSelectedSlot(null)
    setSelectedDate(null)
    setEditingMeal(meal)
    setEditRecipeId(meal.recipeId)
    setEditCustomName(meal.customName)
    setEditServings(meal.servings)
  }

  const cancelEdit = () => {
    setEditingMeal(null)
    setEditRecipeId('')
    setEditCustomName('')
    setEditServings(1)
  }

  const handleSaveEdit = async () => {
    if (!editingMeal) return
    if (!editRecipeId && !editCustomName.trim()) return
    try {
      const req: AddMealInput = {
        planId: plan.id,
        mealDate: editingMeal.mealDate,
        mealSlot: editingMeal.mealSlot,
        recipeId: editRecipeId,
        customName: editCustomName,
        servings: editServings
      }
      await addMeal(req)
      const date = editingMeal.mealDate
      const slot = editingMeal.mealSlot
      cancelEdit()
      onAddMeal(date, slot, editRecipeId, editCustomName, editServings)
    } catch (err) {
      console.error('Failed to edit meal:', err)
    }
  }

  const movingMealName =
    movingMeal?.customName || recipes.find((r) => r.id === movingMeal?.recipeId)?.name || '?'

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <Button variant="secondary" size="sm" onClick={onPrevWeek}>
          ← Prev
        </Button>
        <span className="text-sm font-semibold text-fg text-center">
          {weekDates[0].toLocaleDateString()} – {weekDates[6].toLocaleDateString()}
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

      <div className={`overflow-x-auto${movingMeal ? ' cursor-crosshair' : ''}`}>
        <div
          className="grid gap-1 text-xs"
          style={{ gridTemplateColumns: 'minmax(4.5rem, auto) repeat(7, 1fr)' }}
        >
          <div />
          {dayNames.map((day) => (
            <div key={day} className="py-1 text-center font-semibold text-fg">
              {day}
            </div>
          ))}

          <div />
          {weekDates.map((date) => (
            <div key={formatMealDate(date)} className="py-1 text-center text-muted">
              {date.getDate()}
            </div>
          ))}

          {MEAL_SLOTS.map((slot) => (
            <React.Fragment key={slot}>
              <div className="flex items-center pr-1 text-xs font-medium text-muted">
                {slot.charAt(0).toUpperCase() + slot.slice(1)}
              </div>
              {weekDates.map((date) => {
                const formattedDate = formatMealDate(date)
                const mealsInSlot = getMealsForSlot(formattedDate, slot)
                return (
                  <div
                    key={`${formattedDate}-${slot}`}
                    className={`min-h-8 rounded-lg border p-1 ${movingMeal ? 'hover:border-accent/50 hover:bg-accent/10' : 'border-border'}`}
                    onClick={() => handleCellClick(formattedDate, slot, mealsInSlot.length > 0)}
                  >
                    {mealsInSlot.length > 0 ? (
                      <div className="space-y-1">
                        {mealsInSlot.map((meal) => {
                          const recipe = recipes.find((r) => r.id === meal.recipeId)
                          const isMoving = movingMeal?.id === meal.id
                          return (
                            <div
                              key={meal.id}
                              onClick={(e) => {
                                e.stopPropagation()
                                handleMealClick(meal)
                              }}
                              className={`flex cursor-pointer select-none items-center justify-between gap-1 rounded-lg p-1 ${
                                isMoving
                                  ? 'bg-accent/20 ring-2 ring-accent'
                                  : 'bg-accent/10 hover:bg-accent/20'
                              }`}
                            >
                              <span className="truncate text-fg">
                                {meal.customName || recipe?.name || '?'}
                              </span>
                              {meal.servings > 1 && (
                                <span className="shrink-0 text-muted">×{meal.servings}</span>
                              )}
                              {!movingMeal && (
                                <>
                                  <button
                                    aria-label="Edit meal"
                                    onClick={(e) => {
                                      e.stopPropagation()
                                      handleEditClick(meal)
                                    }}
                                    className="shrink-0 text-xs text-accent hover:text-accent-hover"
                                  >
                                    ✏
                                  </button>
                                  <button
                                    onClick={(e) => {
                                      e.stopPropagation()
                                      handleDeleteMeal(meal.id)
                                    }}
                                    className="shrink-0 font-bold text-danger hover:opacity-80"
                                  >
                                    ×
                                  </button>
                                </>
                              )}
                            </div>
                          )
                        })}
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
              })}
            </React.Fragment>
          ))}
        </div>
      </div>

      {selectedSlot && selectedDate && (
        <div className="rounded-2xl border border-border bg-card p-4 shadow-card space-y-3">
          <h3 className="text-sm font-semibold text-fg">
            Add meal — {selectedSlot.charAt(0).toUpperCase() + selectedSlot.slice(1)},{' '}
            {new Date(selectedDate + 'T00:00:00').toLocaleDateString()}
          </h3>
          <RecipeCombobox
            recipes={recipes}
            onSelect={handleComboboxSelect}
            autoFocus
            onEnter={handleAddMeal}
          />
          <input
            type="number"
            min="1"
            value={selectedServings}
            onChange={(e) => setSelectedServings(parseInt(e.target.value, 10))}
            onKeyDown={(e) => e.key === 'Enter' && handleAddMeal()}
            placeholder="Servings"
            className="flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          />
          <div className="flex gap-2">
            <Button onClick={handleAddMeal} className="flex-1">
              Add
            </Button>
            <Button variant="secondary" onClick={cancelAdd} className="flex-1">
              Cancel
            </Button>
          </div>
        </div>
      )}

      {editingMeal && !selectedSlot && (
        <div className="rounded-2xl border border-border bg-card p-4 shadow-card space-y-3">
          <h3 className="text-sm font-semibold text-fg">
            Edit meal —{' '}
            {editingMeal.mealSlot.charAt(0).toUpperCase() + editingMeal.mealSlot.slice(1)},{' '}
            {new Date(editingMeal.mealDate + 'T00:00:00').toLocaleDateString()}
          </h3>
          <RecipeCombobox
            recipes={recipes}
            initialValue={editCustomName || recipes.find((r) => r.id === editRecipeId)?.name || ''}
            onSelect={(recipeId, customName) => {
              setEditRecipeId(recipeId)
              setEditCustomName(customName)
            }}
            autoFocus
            onEnter={handleSaveEdit}
          />
          <input
            type="number"
            min="1"
            value={editServings}
            onChange={(e) => setEditServings(parseInt(e.target.value, 10))}
            onKeyDown={(e) => e.key === 'Enter' && handleSaveEdit()}
            placeholder="Servings"
            className="flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
          />
          <div className="flex gap-2">
            <Button onClick={handleSaveEdit} className="flex-1">
              Save
            </Button>
            <Button variant="secondary" onClick={cancelEdit} className="flex-1">
              Cancel
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
