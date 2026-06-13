'use client'

import React, { useState, useEffect } from 'react'
import { getWeekDates, formatMealDate, MEAL_SLOTS } from '@/lib/recipes/mealPlanCalendar'
import { useAddMeal, useDeleteMeal, useMoveMeal, useMealSuggestions } from '@/hooks/useMealPlans'
import type { AddMealInput, DeleteMealInput, MoveMealInput } from '@/hooks/useMealPlans'
import type { Plan, PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import MealPlanEntryForm from './MealPlanEntryForm'
import MealPlanMealChip from './MealPlanMealChip'
import { Button } from '@/components/ui/button'
import { formatCustomNameLabel } from '@/lib/customItems'

interface MealPlanCalendarProps {
  plan: Plan
  recipes: Recipe[]
  weekOffset: number
  onPrevWeek: () => void
  onNextWeek: () => void
  onMutate?: () => void
}

export default function MealPlanCalendar({
  plan,
  recipes,
  weekOffset,
  onPrevWeek,
  onNextWeek,
  onMutate
}: MealPlanCalendarProps) {
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [swappingMeal, setSwappingMeal] = useState<PlanMeal | null>(null)
  const [editingMeal, setEditingMeal] = useState<PlanMeal | null>(null)

  const addMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()
  const moveMeal = useMoveMeal()

  // Suggestions only load while adding (selectedDate + selectedSlot set); the
  // hook's key is null otherwise. Map returned IDs to the recipes we already
  // have, dropping any that are no longer available.
  const { data: suggestData } = useMealSuggestions(plan.id, selectedDate ?? '', selectedSlot ?? '')
  const suggestedRecipes = (suggestData?.recipeIds ?? [])
    .map((id) => recipes.find((r) => r.id === id))
    .filter((r): r is Recipe => r !== undefined)

  const weekDates = getWeekDates(weekOffset)
  const DAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
  const today = formatMealDate(new Date())

  // Each slot holds at most one meal; return the first match if present.
  const getMealForSlot = (date: string, slot: string) =>
    (plan.meals || []).find((m) => m.mealDate === date && m.mealSlot === slot)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (swappingMeal) setSwappingMeal(null)
        if (editingMeal) setEditingMeal(null)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [swappingMeal, editingMeal])

  const handleSaveAdd = async (
    recipeId: string,
    customName: string,
    servings: number,
    excludeFromShoppingList: boolean
  ) => {
    if (!selectedSlot || !selectedDate) return
    try {
      const req: AddMealInput = {
        planId: plan.id,
        mealDate: selectedDate,
        mealSlot: selectedSlot,
        recipeId,
        customName,
        servings,
        excludeFromShoppingList
      }
      await addMeal(req)
      setSelectedSlot(null)
      setSelectedDate(null)
      onMutate?.()
    } catch (err) {
      console.error('Failed to add meal:', err)
    }
  }

  const handleDeleteMeal = async (mealId: string) => {
    try {
      const req: DeleteMealInput = { planId: plan.id, mealId }
      await deleteMeal(req)
      onMutate?.()
    } catch (err) {
      console.error('Failed to delete meal:', err)
    }
  }

  const handleStartSwap = (meal: PlanMeal) => {
    setSwappingMeal(meal)
    setSelectedSlot(null)
    setSelectedDate(null)
  }

  const handleMealClick = (meal: PlanMeal) => {
    if (!swappingMeal) return
    if (swappingMeal.id === meal.id) {
      setSwappingMeal(null)
      return
    }
    handlePlaceSwap(meal.mealDate, meal.mealSlot)
  }

  const handleCellClick = (date: string, slot: string) => {
    if (swappingMeal) {
      handlePlaceSwap(date, slot)
      return
    }
    if (!getMealForSlot(date, slot)) {
      setSelectedSlot(slot)
      setSelectedDate(date)
    }
  }

  // Move the picked meal to the target slot. If the target slot already holds a
  // meal, the two trade places so each slot keeps a single entry.
  const handlePlaceSwap = async (newDate: string, newSlot: string) => {
    if (!swappingMeal) return
    if (swappingMeal.mealDate === newDate && swappingMeal.mealSlot === newSlot) {
      setSwappingMeal(null)
      return
    }
    try {
      const targetMeal = getMealForSlot(newDate, newSlot)
      const target = targetMeal && targetMeal.id !== swappingMeal.id ? targetMeal : undefined
      const moveSelf: MoveMealInput = {
        planId: plan.id,
        mealId: swappingMeal.id,
        newDate,
        newSlot
      }
      await moveMeal(moveSelf)
      if (target) {
        const moveTarget: MoveMealInput = {
          planId: plan.id,
          mealId: target.id,
          newDate: swappingMeal.mealDate,
          newSlot: swappingMeal.mealSlot
        }
        await moveMeal(moveTarget)
      }
      setSwappingMeal(null)
      onMutate?.()
    } catch (err) {
      console.error('Failed to swap meal:', err)
    }
  }

  const handleEditClick = (meal: PlanMeal) => {
    setSwappingMeal(null)
    setSelectedSlot(null)
    setSelectedDate(null)
    setEditingMeal(meal)
  }

  const handleSaveEdit = async (
    recipeId: string,
    customName: string,
    servings: number,
    excludeFromShoppingList: boolean
  ) => {
    if (!editingMeal) return
    try {
      await deleteMeal({ planId: plan.id, mealId: editingMeal.id })
      const req: AddMealInput = {
        planId: plan.id,
        mealDate: editingMeal.mealDate,
        mealSlot: editingMeal.mealSlot,
        recipeId,
        customName,
        servings,
        excludeFromShoppingList
      }
      await addMeal(req)
      setEditingMeal(null)
      onMutate?.()
    } catch (err) {
      console.error('Failed to edit meal:', err)
    }
  }

  const swappingMealName = swappingMeal?.customName
    ? formatCustomNameLabel(swappingMeal.customName)
    : recipes.find((r) => r.id === swappingMeal?.recipeId)?.name || '?'

  const renderCell = (formattedDate: string, slot: string) => {
    const meal = getMealForSlot(formattedDate, slot)
    return (
      <div
        key={`${formattedDate}-${slot}`}
        className={`min-h-14 min-w-0 rounded-xl border p-1.5 ${swappingMeal ? 'hover:border-accent/50 hover:bg-accent/10' : 'border-border'}`}
        onClick={() => handleCellClick(formattedDate, slot)}
      >
        {meal ? (
          <MealPlanMealChip
            meal={meal}
            recipe={recipes.find((r) => r.id === meal.recipeId)}
            isSwapping={swappingMeal?.id === meal.id}
            inSwapMode={!!swappingMeal}
            onMealClick={handleMealClick}
            onSwapClick={handleStartSwap}
            onEditClick={handleEditClick}
            onDeleteMeal={handleDeleteMeal}
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
              setSelectedSlot(slot)
              setSelectedDate(formattedDate)
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

      {swappingMeal && (
        <div className="fixed inset-x-0 bottom-4 z-30 mx-auto flex w-fit max-w-[calc(100vw-2rem)] items-center gap-3 rounded-2xl border border-accent/30 bg-card px-4 py-2.5 text-sm text-accent shadow-elevated">
          <span className="min-w-0 truncate">
            Swapping <strong>{swappingMealName}</strong> — tap another meal or an empty cell
          </span>
          <Button
            variant="secondary"
            size="sm"
            className="shrink-0"
            onClick={() => setSwappingMeal(null)}
          >
            Cancel
          </Button>
        </div>
      )}

      <div className="flex flex-col gap-4 items-start">
        <div className="w-full min-w-0">
          {/* Mobile: stacked by day */}
          <div className={`sm:hidden space-y-3 text-xs${swappingMeal ? ' cursor-crosshair' : ''}`}>
            {weekDates.map((date) => {
              const formattedDate = formatMealDate(date)
              const isToday = formattedDate === today
              return (
                <div key={formattedDate} className="rounded-xl border border-border p-2">
                  <div
                    className={`mb-2 font-semibold text-sm${isToday ? ' text-accent' : ' text-fg'}`}
                  >
                    {DAY_NAMES[date.getDay()]} {date.getDate()}
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
          <div
            className={`hidden sm:block overflow-x-auto${swappingMeal ? ' cursor-crosshair' : ''}`}
          >
            <div
              className="grid gap-1.5 text-sm"
              style={{ gridTemplateColumns: 'minmax(5rem, auto) repeat(7, 1fr)' }}
            >
              <div />
              {weekDates.map((date) => {
                const isToday = formatMealDate(date) === today
                return (
                  <div
                    key={formatMealDate(date)}
                    className={`py-1 text-center font-semibold${isToday ? ' text-accent' : ' text-fg'}`}
                  >
                    {DAY_NAMES[date.getDay()]}
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

      {(() => {
        const isAdding = !!(selectedSlot && selectedDate)
        const isEditing = !!editingMeal && !selectedSlot
        const handleCancel = () => {
          setSelectedSlot(null)
          setSelectedDate(null)
          setEditingMeal(null)
        }
        const formTitle = isAdding
          ? `Add meal — ${selectedSlot!.charAt(0).toUpperCase() + selectedSlot!.slice(1)}, ${new Date(selectedDate! + 'T00:00:00').toLocaleDateString()}`
          : isEditing
            ? `Edit meal — ${editingMeal!.mealSlot.charAt(0).toUpperCase() + editingMeal!.mealSlot.slice(1)}, ${new Date(editingMeal!.mealDate + 'T00:00:00').toLocaleDateString()}`
            : ''
        return (
          <MealPlanEntryForm
            key={
              isAdding
                ? `add-${selectedDate}-${selectedSlot}`
                : isEditing
                  ? `edit-${editingMeal!.id}`
                  : 'closed'
            }
            open={isAdding || isEditing}
            title={formTitle}
            recipes={recipes}
            suggestedRecipes={isAdding ? suggestedRecipes : []}
            initialRecipeId={isEditing ? editingMeal!.recipeId : ''}
            initialCustomName={isEditing ? editingMeal!.customName : ''}
            initialServings={isEditing ? editingMeal!.servings : 1}
            initialExcludeFromShoppingList={
              isEditing ? editingMeal!.excludeFromShoppingList : false
            }
            saveLabel={isAdding ? 'Add' : 'Save'}
            onSave={isAdding ? handleSaveAdd : handleSaveEdit}
            onCancel={handleCancel}
          />
        )
      })()}
    </div>
  )
}
