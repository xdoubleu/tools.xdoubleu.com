'use client'

import { getWeekDates } from '@/lib/recipes/mealPlanCalendar'
import type { Plan } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import MealPlanEntryForm from './MealPlanEntryForm'
import MealPlanWeekGrid from './MealPlanWeekGrid'
import { useMealCalendarState } from './useMealCalendarState'
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
  const {
    selectedSlot,
    selectedDate,
    swappingMeal,
    setSwappingMeal,
    editingMeal,
    fillingDate,
    suggestedRecipes,
    getMealForSlot,
    startAdd,
    startFillDay,
    cancelForm,
    handleSaveAdd,
    handleSaveEdit,
    handleSaveFillDay,
    handleDeleteMeal,
    handleStartSwap,
    handleMealClick,
    handleCellClick,
    handleEditClick
  } = useMealCalendarState(plan, recipes, onMutate)

  const weekDates = getWeekDates(weekOffset)

  const swappingMealName = swappingMeal?.customName
    ? formatCustomNameLabel(swappingMeal.customName)
    : recipes.find((r) => r.id === swappingMeal?.recipeId)?.name || '?'

  const isAdding = !!(selectedSlot && selectedDate)
  const isEditing = !!editingMeal && !selectedSlot
  const isFilling = !!fillingDate
  const formTitle = isAdding
    ? `Add meal — ${selectedSlot!.charAt(0).toUpperCase() + selectedSlot!.slice(1)}, ${new Date(selectedDate! + 'T00:00:00').toLocaleDateString()}`
    : isEditing
      ? `Edit meal — ${editingMeal!.mealSlot.charAt(0).toUpperCase() + editingMeal!.mealSlot.slice(1)}, ${new Date(editingMeal!.mealDate + 'T00:00:00').toLocaleDateString()}`
      : isFilling
        ? `Fill day — ${new Date(fillingDate! + 'T00:00:00').toLocaleDateString()}`
        : ''

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

      <MealPlanWeekGrid
        weekDates={weekDates}
        recipes={recipes}
        swappingMeal={swappingMeal}
        getMealForSlot={getMealForSlot}
        onCellClick={handleCellClick}
        onMealClick={handleMealClick}
        onStartSwap={handleStartSwap}
        onEditClick={handleEditClick}
        onDeleteMeal={handleDeleteMeal}
        onAddClick={startAdd}
        onFillDay={startFillDay}
      />

      <MealPlanEntryForm
        key={
          isAdding
            ? `add-${selectedDate}-${selectedSlot}`
            : isEditing
              ? `edit-${editingMeal!.id}`
              : isFilling
                ? `fill-${fillingDate}`
                : 'closed'
        }
        open={isAdding || isEditing || isFilling}
        title={formTitle}
        recipes={recipes}
        suggestedRecipes={isAdding ? suggestedRecipes : []}
        initialRecipeId={isEditing ? editingMeal!.recipeId : ''}
        initialCustomName={isEditing ? editingMeal!.customName : ''}
        initialServings={isEditing ? editingMeal!.servings : 1}
        initialExcludeFromShoppingList={isEditing ? editingMeal!.excludeFromShoppingList : false}
        saveLabel={isAdding ? 'Add' : isFilling ? 'Fill day' : 'Save'}
        onSave={isAdding ? handleSaveAdd : isFilling ? handleSaveFillDay : handleSaveEdit}
        onCancel={cancelForm}
      />
    </div>
  )
}
