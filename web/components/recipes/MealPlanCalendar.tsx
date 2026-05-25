'use client'

import React, { useState } from 'react'
import { getWeekDates, formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'
import { useAddMeal, useDeleteMeal } from '@/hooks/useRecipes'
import { AddMealRequest, DeleteMealRequest } from '@/lib/gen/recipes/v1/mealplans_pb'
import type { Plan } from '@/lib/gen/recipes/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import RecipeCombobox from './RecipeCombobox'

interface MealPlanCalendarProps {
  plan: Plan
  recipes: Recipe[]
  onAddMeal: (
    date: string,
    slot: string,
    recipeId: string,
    customName: string,
    servings: number
  ) => void
  onDeleteMeal: (mealId: string) => void
}

export default function MealPlanCalendar({
  plan,
  recipes,
  onAddMeal,
  onDeleteMeal
}: MealPlanCalendarProps) {
  const [weekOffset, setWeekOffset] = useState(0)
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [selectedRecipeId, setSelectedRecipeId] = useState('')
  const [selectedCustomName, setSelectedCustomName] = useState('')
  const [selectedServings, setSelectedServings] = useState(1)

  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()

  const weekDates = getWeekDates(weekOffset)
  const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

  const getMealsForSlot = (date: string, slot: string) =>
    (plan.meals || []).filter((m) => m.mealDate === date && m.mealSlot === slot)

  const handleComboboxSelect = (recipeId: string, customName: string) => {
    setSelectedRecipeId(recipeId)
    setSelectedCustomName(customName)
  }

  const handleAddMeal = async () => {
    if (!selectedSlot || !selectedDate) return
    if (!selectedRecipeId && !selectedCustomName.trim()) return
    try {
      await addMeal(
        new AddMealRequest({
          planId: plan.id,
          mealDate: selectedDate,
          mealSlot: selectedSlot,
          recipeId: selectedRecipeId,
          customName: selectedCustomName,
          servings: selectedServings
        })
      )
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
      await deleteMeal(new DeleteMealRequest({ planId: plan.id, mealId }))
      onDeleteMeal(mealId)
    } catch (err) {
      console.error('Failed to delete meal:', err)
    }
  }

  const cancelAdd = () => {
    setSelectedSlot(null)
    setSelectedDate(null)
    setSelectedRecipeId('')
    setSelectedCustomName('')
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <button
          onClick={() => setWeekOffset(weekOffset - 1)}
          className="px-4 py-2 bg-subtle text-bg rounded hover:bg-fg"
        >
          Previous Week
        </button>
        <span className="font-semibold">
          {weekDates[0].toLocaleDateString()} - {weekDates[6].toLocaleDateString()}
        </span>
        <button
          onClick={() => setWeekOffset(weekOffset + 1)}
          className="px-4 py-2 bg-subtle text-bg rounded hover:bg-fg"
        >
          Next Week
        </button>
      </div>

      <div className="overflow-x-auto">
        <div
          className="grid gap-1 text-xs"
          style={{ gridTemplateColumns: 'minmax(4.5rem, auto) repeat(7, 1fr)' }}
        >
          {/* Header row: empty corner + day names */}
          <div />
          {dayNames.map((day) => (
            <div key={day} className="font-semibold text-center py-1">
              {day}
            </div>
          ))}

          {/* Date row: empty + date numbers */}
          <div />
          {weekDates.map((date) => (
            <div key={formatMealDate(date)} className="text-center text-muted py-1">
              {date.getDate()}
            </div>
          ))}

          {/* Slot rows */}
          {MEAL_SLOTS.map((slot) => (
            <React.Fragment key={slot}>
              <div className="text-xs font-medium text-muted flex items-center pr-1">
                {slot.charAt(0).toUpperCase() + slot.slice(1)}
              </div>
              {weekDates.map((date) => {
                const formattedDate = formatMealDate(date)
                const mealsInSlot = getMealsForSlot(formattedDate, slot)
                return (
                  <div key={`${formattedDate}-${slot}`} className="border rounded p-1 min-h-[2rem]">
                    {mealsInSlot.length > 0 ? (
                      <div className="space-y-1">
                        {mealsInSlot.map((meal) => {
                          const recipe = recipes.find((r) => r.id === meal.recipeId)
                          return (
                            <div
                              key={meal.id}
                              className="bg-blue-50 p-1 rounded flex items-center justify-between gap-1"
                            >
                              <span className="truncate">
                                {meal.customName || recipe?.name || '?'}
                              </span>
                              {meal.servings > 1 && (
                                <span className="text-muted shrink-0">×{meal.servings}</span>
                              )}
                              <button
                                onClick={() => handleDeleteMeal(meal.id)}
                                className="text-red-600 hover:text-red-800 font-bold shrink-0"
                              >
                                ×
                              </button>
                            </div>
                          )
                        })}
                      </div>
                    ) : (
                      <button
                        onClick={() => {
                          setSelectedSlot(slot)
                          setSelectedDate(formattedDate)
                        }}
                        className="w-full h-full text-center bg-surface hover:bg-border p-1 rounded"
                      >
                        +
                      </button>
                    )}
                  </div>
                )
              })}
            </React.Fragment>
          ))}
        </div>
      </div>

      {selectedSlot && selectedDate && (
        <div className="border border-border rounded p-4 bg-card space-y-3">
          <h3 className="font-semibold text-sm">
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
            className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text"
          />
          <div className="flex gap-2">
            <button
              onClick={handleAddMeal}
              className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
            >
              Add
            </button>
            <button
              onClick={cancelAdd}
              className="flex-1 px-4 py-2 bg-subtle text-bg rounded hover:bg-fg"
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
