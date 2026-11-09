import { useState, useEffect, useCallback } from 'react'
import {
  useAddMeal,
  useUpdateMeal,
  useDeleteMeal,
  useMoveMeal,
  useMealSuggestions
} from '@/hooks/useMealPlans'
import type {
  AddMealInput,
  UpdateMealInput,
  DeleteMealInput,
  MoveMealInput
} from '@/hooks/useMealPlans'
import type { Plan, PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { MEAL_SLOTS } from '@/lib/mealplans/mealPlanCalendar'

export interface MealSuggestion {
  recipe: Recipe
  servings: number
  /** Rarely or never planned, as opposed to planned before in this cell. */
  fresh: boolean
}

export type PickMode = 'move' | 'swap'

// useMealCalendarState owns the calendar's add/move/swap/edit state so the
// calendar stays presentational.
export function useMealCalendarState(plan: Plan, recipes: Recipe[], onMutate?: () => void) {
  const [selectedSlot, setSelectedSlot] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  // The picked meal: `move` relocates it alone, `swap` exchanges its whole slot.
  const [swappingMeal, setSwappingMeal] = useState<PlanMeal | null>(null)
  const [pickMode, setPickMode] = useState<PickMode>('swap')
  const [editingMeal, setEditingMeal] = useState<PlanMeal | null>(null)
  const [fillingDate, setFillingDate] = useState<string | null>(null)
  // The chip to refocus once swap mode ends, since its target control unmounts.
  const [focusMealId, setFocusMealId] = useState('')
  const clearFocusMeal = useCallback(() => setFocusMealId(''), [])
  const cancelSwap = (meal: PlanMeal) => {
    setFocusMealId(meal.id)
    setSwappingMeal(null)
  }

  const createMeal = useAddMeal()
  const updateMeal = useUpdateMeal()
  const deleteMeal = useDeleteMeal()
  const moveMeal = useMoveMeal()

  // Suggestions load only while adding; unavailable recipes are dropped and
  // each carries its most-used servings and whether it is a fresh pick.
  const { data: suggestData } = useMealSuggestions(plan.id, selectedDate ?? '', selectedSlot ?? '')
  const suggestedRecipes: MealSuggestion[] = (suggestData?.suggestions ?? [])
    .map((s) => {
      const recipe = recipes.find((r) => r.id === s.recipeId)
      return recipe ? { recipe, servings: s.servings, fresh: s.fresh } : undefined
    })
    .filter((s): s is MealSuggestion => s !== undefined)

  const recipeName = (id: string) => recipes.find((r) => r.id === id)?.name

  // A slot can hold any number of meals.
  const getMealsForSlot = (date: string, slot: string) =>
    (plan.meals || []).filter((m) => m.mealDate === date && m.mealSlot === slot)

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (swappingMeal) cancelSwap(swappingMeal)
        if (editingMeal) setEditingMeal(null)
        if (fillingDate) setFillingDate(null)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [swappingMeal, editingMeal, fillingDate])

  const startAdd = (date: string, slot: string) => {
    setSelectedSlot(slot)
    setSelectedDate(date)
  }

  const startFillDay = (date: string) => {
    setFillingDate(date)
    setSelectedSlot(null)
    setSelectedDate(null)
    setEditingMeal(null)
    setSwappingMeal(null)
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
      await createMeal(req, recipeName(recipeId))
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

  const handleStartPick = (meal: PlanMeal, mode: PickMode) => {
    setPickMode(mode)
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
    handlePlace(meal.mealDate, meal.mealSlot)
  }

  const handleCellClick = (date: string, slot: string) => {
    if (swappingMeal) {
      handlePlace(date, slot)
      return
    }
    startAdd(date, slot)
  }

  // Move relocates the picked meal alone; swap exchanges every meal of its
  // slot with every meal of the target slot, empty or not.
  const handlePlace = async (newDate: string, newSlot: string) => {
    if (!swappingMeal) return
    const { mealDate, mealSlot } = swappingMeal
    if (mealDate === newDate && mealSlot === newSlot) {
      setSwappingMeal(null)
      return
    }
    try {
      const toMove: MoveMealInput[] = []
      if (pickMode === 'move') {
        toMove.push({ planId: plan.id, mealId: swappingMeal.id, newDate, newSlot })
      } else {
        for (const m of getMealsForSlot(mealDate, mealSlot)) {
          toMove.push({ planId: plan.id, mealId: m.id, newDate, newSlot })
        }
        for (const m of getMealsForSlot(newDate, newSlot)) {
          toMove.push({ planId: plan.id, mealId: m.id, newDate: mealDate, newSlot: mealSlot })
        }
      }
      for (const req of toMove) await moveMeal(req)
      setFocusMealId(swappingMeal.id)
      setSwappingMeal(null)
      onMutate?.()
    } catch (err) {
      console.error('Failed to move meal:', err)
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
      const req: UpdateMealInput = {
        planId: plan.id,
        mealId: editingMeal.id,
        recipeId,
        customName,
        servings,
        excludeFromShoppingList
      }
      await updateMeal(req, recipeName(recipeId))
      setEditingMeal(null)
      onMutate?.()
    } catch (err) {
      console.error('Failed to edit meal:', err)
    }
  }

  // Replaces every slot of a day; clears existing meals first since a slot
  // can hold several.
  const handleSaveFillDay = async (
    recipeId: string,
    customName: string,
    servings: number,
    excludeFromShoppingList: boolean
  ) => {
    if (!fillingDate) return
    try {
      for (const slot of MEAL_SLOTS) {
        const existing = (plan.meals || []).filter(
          (m) => m.mealDate === fillingDate && m.mealSlot === slot
        )
        for (const meal of existing) {
          await deleteMeal({ planId: plan.id, mealId: meal.id })
        }
        const req: AddMealInput = {
          planId: plan.id,
          mealDate: fillingDate,
          mealSlot: slot,
          recipeId,
          customName,
          servings,
          excludeFromShoppingList
        }
        await createMeal(req, recipeName(recipeId))
      }
      setFillingDate(null)
      onMutate?.()
    } catch (err) {
      console.error('Failed to fill day:', err)
    }
  }

  const cancelForm = () => {
    setSelectedSlot(null)
    setSelectedDate(null)
    setEditingMeal(null)
    setFillingDate(null)
  }

  return {
    selectedSlot,
    selectedDate,
    swappingMeal,
    pickMode,
    cancelSwap,
    focusMealId,
    clearFocusMeal,
    editingMeal,
    fillingDate,
    suggestedRecipes,
    getMealsForSlot,
    startAdd,
    startFillDay,
    cancelForm,
    handleSaveAdd,
    handleSaveEdit,
    handleSaveFillDay,
    handleDeleteMeal,
    handleStartPick,
    handleMealClick,
    handleCellClick,
    handleEditClick
  }
}
