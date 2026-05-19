'use client'

import { useState } from 'react'
import { getWeekDates, formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'
import { useAddMeal, useDeleteMeal } from '@/hooks/useRecipes'
import { getApiUrl } from '@/lib/env'
import { AddMealRequest, DeleteMealRequest } from '@/lib/gen/recipes/v1/mealplans_pb'
import type { Plan } from '@/lib/gen/recipes/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

interface MealPlanCalendarProps {
  plan: Plan
  recipes: Recipe[]
  onAddMeal: (date: string, slot: string, recipeId: string, servings: number) => void
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
  const [selectedServings, setSelectedServings] = useState(1)

  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()

  const weekDates = getWeekDates(weekOffset)
  const dayNames = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

  const getMealsForSlot = (date: string, slot: string) => {
    return (plan.meals || []).filter((m) => m.mealDate === date && m.mealSlot === slot)
  }

  const handleAddMeal = async () => {
    if (selectedSlot && selectedDate && selectedRecipeId) {
      try {
        await addMeal(
          new AddMealRequest({
            planId: plan.id,
            mealDate: selectedDate,
            mealSlot: selectedSlot,
            recipeId: selectedRecipeId
          })
        )
        setSelectedSlot(null)
        setSelectedDate(null)
        setSelectedRecipeId('')
        onAddMeal(selectedDate, selectedSlot, selectedRecipeId, selectedServings)
      } catch (err) {
        console.error('Failed to add meal:', err)
      }
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

  const icalUrl = `${getApiUrl()}/recipes/api/plans/${plan.id}/ical`

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
        <div className="grid gap-2" style={{ gridTemplateColumns: 'repeat(7, 1fr)' }}>
          {dayNames.map((day) => (
            <div key={day} className="font-semibold text-center text-sm">
              {day}
            </div>
          ))}

          {weekDates.map((date) => (
            <div key={formatMealDate(date)} className="space-y-1">
              <div className="text-xs text-muted text-center">{date.getDate()}</div>
              {MEAL_SLOTS.map((slot) => {
                const mealsInSlot = getMealsForSlot(formatMealDate(date), slot)
                const formattedDate = formatMealDate(date)

                return (
                  <div key={`${formattedDate}-${slot}`} className="border rounded p-1 text-xs">
                    {mealsInSlot.length > 0 ? (
                      <div className="space-y-1">
                        {mealsInSlot.map((meal) => {
                          const recipe = recipes.find((r) => r.id === meal.recipeId)
                          return (
                            <div
                              key={meal.id}
                              className="bg-blue-50 p-1 rounded flex items-center justify-between gap-1"
                            >
                              <span className="truncate">{recipe?.name}</span>
                              <button
                                onClick={() => handleDeleteMeal(meal.id)}
                                className="text-red-600 hover:text-red-800 font-bold"
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
                        className="w-full text-center bg-surface hover:bg-border p-1 rounded"
                      >
                        +
                      </button>
                    )}
                  </div>
                )
              })}
            </div>
          ))}
        </div>
      </div>

      {selectedSlot && selectedDate && (
        <div className="border border-border rounded p-4 bg-card space-y-3">
          <h3 className="font-semibold">Add meal to {selectedSlot}</h3>
          <select
            value={selectedRecipeId}
            onChange={(e) => setSelectedRecipeId(e.target.value)}
            className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text"
          >
            <option value="">Select recipe...</option>
            {recipes.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
          <input
            type="number"
            min="1"
            value={selectedServings}
            onChange={(e) => setSelectedServings(parseInt(e.target.value, 10))}
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
              onClick={() => {
                setSelectedSlot(null)
                setSelectedDate(null)
              }}
              className="flex-1 px-4 py-2 bg-subtle text-bg rounded hover:bg-fg"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      <div className="p-4 border border-border rounded bg-surface">
        <p className="text-sm text-muted mb-2">iCal Export:</p>
        <code className="text-xs break-all bg-card p-2 rounded border border-border">
          {icalUrl}
        </code>
        <button
          onClick={() => {
            navigator.clipboard.writeText(icalUrl)
          }}
          className="mt-2 px-3 py-1 bg-blue-600 text-white text-sm rounded hover:bg-blue-700"
        >
          Copy URL
        </button>
      </div>
    </div>
  )
}
