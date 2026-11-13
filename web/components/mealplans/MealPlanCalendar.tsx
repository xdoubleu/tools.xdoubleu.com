import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { getWeekDates } from '@/lib/mealplans/mealPlanCalendar'
import type { Plan, PlanMeal } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import MealPlanEntryForm from './MealPlanEntryForm'
import MealPlanWeekGrid from './MealPlanWeekGrid'
import { useMealCalendarState } from './useMealCalendarState'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { ConfirmDialog } from '@/components/ui/dialog'
import { formatCustomNameLabel } from '@/lib/customItems'
import { formatDate, formatDateRange } from '@/lib/dates'

interface MealPlanCalendarProps {
  plan: Plan
  recipes: Recipe[]
  weekOffset: number
  onPrevWeek: () => void
  onNextWeek: () => void
  onThisWeek: () => void
  onMutate?: () => void
}

export default function MealPlanCalendar({
  plan,
  recipes,
  weekOffset,
  onPrevWeek,
  onNextWeek,
  onThisWeek,
  onMutate
}: MealPlanCalendarProps) {
  const {
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
  } = useMealCalendarState(plan, recipes, onMutate)

  // Kept after the confirm closes so its title doesn't change mid-animation.
  const [mealToDelete, setMealToDelete] = useState({ id: '', name: '' })
  const [confirmingDelete, setConfirmingDelete] = useState(false)

  const weekDates = getWeekDates(weekOffset)

  const mealName = (meal: PlanMeal | null | undefined) =>
    meal?.customName
      ? formatCustomNameLabel(meal.customName)
      : recipes.find((r) => r.id === meal?.recipeId)?.name || meal?.recipeRef?.name || '?'
  const swappingMealName = mealName(swappingMeal)

  const confirmDeleteMeal = (mealId: string) => {
    setMealToDelete({ id: mealId, name: mealName(plan.meals.find((m) => m.id === mealId)) })
    setConfirmingDelete(true)
  }

  const isAdding = !!(selectedSlot && selectedDate)
  const isEditing = !!editingMeal && !selectedSlot
  const isFilling = !!fillingDate
  const formTitle = isAdding
    ? `Add meal — ${selectedSlot!.charAt(0).toUpperCase() + selectedSlot!.slice(1)}, ${formatDate(selectedDate!)}`
    : isEditing
      ? `Edit meal — ${editingMeal!.mealSlot.charAt(0).toUpperCase() + editingMeal!.mealSlot.slice(1)}, ${formatDate(editingMeal!.mealDate)}`
      : isFilling
        ? `Fill day — ${formatDate(fillingDate!)}`
        : ''

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <Button variant="secondary" size="icon" aria-label="Previous week" onClick={onPrevWeek}>
          <ChevronLeft aria-hidden className="size-4" />
        </Button>
        <div className="flex min-w-0 flex-wrap items-center justify-center gap-x-3 gap-y-1">
          <span className="text-center text-sm font-semibold text-fg">
            {formatDateRange(weekDates[0], weekDates[6])}
          </span>
          <Button variant="secondary" disabled={weekOffset === 0} onClick={onThisWeek}>
            This week
          </Button>
        </div>
        <Button variant="secondary" size="icon" aria-label="Next week" onClick={onNextWeek}>
          <ChevronRight aria-hidden className="size-4" />
        </Button>
      </div>

      {swappingMeal && (
        <Card
          padding="sm"
          className="fixed inset-x-0 bottom-[calc(var(--tab-bar-height)+1rem+env(safe-area-inset-bottom))] z-30 mx-auto lg:bottom-[calc(1rem+env(safe-area-inset-bottom))] flex w-fit max-w-[calc(100vw-2rem)] items-center gap-3 border-accent/30 text-sm text-accent shadow-elevated"
        >
          <span className="min-w-0">
            {pickMode === 'move' ? 'Moving' : 'Swapping'}{' '}
            <strong>
              {pickMode === 'move'
                ? swappingMealName
                : `${swappingMeal.mealSlot} on ${formatDate(swappingMeal.mealDate)}`}
            </strong>{' '}
            — tap another meal or an empty cell
          </span>
          <Button
            variant="secondary"
            size="sm"
            className="shrink-0"
            onClick={() => cancelSwap(swappingMeal)}
          >
            Cancel
          </Button>
        </Card>
      )}

      <MealPlanWeekGrid
        weekDates={weekDates}
        recipes={recipes}
        swappingMeal={swappingMeal}
        pickMode={pickMode}
        getMealsForSlot={getMealsForSlot}
        onCellClick={handleCellClick}
        onMealClick={handleMealClick}
        onStartMove={(meal) => handleStartPick(meal, 'move')}
        onStartSwap={(meal) => handleStartPick(meal, 'swap')}
        onEditClick={handleEditClick}
        onDeleteMeal={confirmDeleteMeal}
        onAddClick={startAdd}
        onFillDay={startFillDay}
        focusMealId={focusMealId}
        onMealFocused={clearFocusMeal}
      />

      {/* Room to scroll the last cells clear of the pick banner. */}
      {swappingMeal && <div aria-hidden className="h-24" />}

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

      <ConfirmDialog
        open={confirmingDelete}
        onOpenChange={setConfirmingDelete}
        title={`Delete ${mealToDelete.name}?`}
        description="It's removed from this plan."
        confirmLabel="Delete"
        destructive
        onConfirm={() => {
          setConfirmingDelete(false)
          void handleDeleteMeal(mealToDelete.id)
        }}
      />
    </div>
  )
}
