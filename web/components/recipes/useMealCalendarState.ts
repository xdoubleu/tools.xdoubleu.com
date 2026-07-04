'use client'

import { useState, useEffect } from 'react'
import { useAddMeal, useDeleteMeal, useMoveMeal, useMealSuggestions } from '@/hooks/useMealPlans'
import type { AddMealInput, DeleteMealInput, MoveMealInput } from '@/hooks/useMealPlans'
import type { Plan, PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

// useMealCalendarState owns the calendar's interaction state machine: adding a
// meal to an empty slot, swapping two meals, and editing an existing meal. The
// calendar component stays purely presentational.
export function useMealCalendarState(plan: Plan, recipes: Recipe[], onMutate?: () => void) {
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [swappingMeal, setSwappingMeal] = useState<PlanMeal | null>(null)
  const [editingMeal, setEditingMeal] = useState<PlanMeal | null>(null)

  const createMeal = useAddMeal()
  const deleteMeal = useDeleteMeal()
  const moveMeal = useMoveMeal()

  // Suggestions only load while adding (selectedDate + selectedSlot set); the
  // hook's key is null otherwise. Map returned IDs to the recipes we already
  // have, dropping any that are no longer available.
  const { data: suggestData } = useMealSuggestions(plan.id, selectedDate ?? '', selectedSlot ?? '')
  const suggestedRecipes = (suggestData?.recipeIds ?? [])
    .map((id) => recipes.find((r) => r.id === id))
    .filter((r): r is Recipe => r !== undefined)

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

  const startAdd = (date: string, slot: string) => {
    setSelectedSlot(slot)
    setSelectedDate(date)
  }

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
      await createMeal(req)
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
      startAdd(date, slot)
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
      await createMeal(req)
      setEditingMeal(null)
      onMutate?.()
    } catch (err) {
      console.error('Failed to edit meal:', err)
    }
  }

  const cancelForm = () => {
    setSelectedSlot(null)
    setSelectedDate(null)
    setEditingMeal(null)
  }

  return {
    selectedSlot,
    selectedDate,
    swappingMeal,
    setSwappingMeal,
    editingMeal,
    suggestedRecipes,
    getMealForSlot,
    startAdd,
    cancelForm,
    handleSaveAdd,
    handleSaveEdit,
    handleDeleteMeal,
    handleStartSwap,
    handleMealClick,
    handleCellClick,
    handleEditClick
  }
}
