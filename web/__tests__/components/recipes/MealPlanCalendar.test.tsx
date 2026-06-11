import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@/hooks/useMealPlans', () => ({
  useAddMeal: jest.fn(),
  useDeleteMeal: jest.fn(),
  useMoveMeal: jest.fn()
}))
jest.mock('@/lib/env', () => ({ getApiUrl: () => 'http://localhost' }))
jest.mock('@/lib/recipes/mealPlanCalendar', () => {
  const week = Array.from({ length: 7 }, (_, i) => {
    const d = new Date('2026-05-25')
    d.setDate(d.getDate() + i)
    return d
  })
  return {
    MEAL_SLOTS: ['breakfast'],
    getWeekDates: () => week,
    formatMealDate: (d: Date) => d.toISOString().slice(0, 10)
  }
})

import { useAddMeal, useDeleteMeal, useMoveMeal } from '@/hooks/useMealPlans'
import MealPlanCalendar from '@/components/recipes/MealPlanCalendar'
import { create } from '@bufbuild/protobuf'
import { PlanSchema, PlanMealSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import { RecipeSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const mockAddMeal = jest.fn()
const mockDeleteMeal = jest.fn()
const mockMoveMeal = jest.fn()

const basePlan = create(PlanSchema, {
  id: 'plan-1',
  name: 'Test Plan',
  canEdit: true
})
const baseRecipes = [create(RecipeSchema, { id: 'r1', name: 'Pasta' })]

function makePlanMeal(overrides: {
  id: string
  mealDate: string
  mealSlot: string
  recipeId: string
  customName: string
  servings: number
  excludeFromShoppingList?: boolean
}) {
  return create(PlanMealSchema, overrides)
}

const mockOnPrevWeek = jest.fn()
const mockOnNextWeek = jest.fn()
const defaultNavProps = {
  weekOffset: 0,
  onPrevWeek: mockOnPrevWeek,
  onNextWeek: mockOnNextWeek
}

beforeEach(() => {
  jest.clearAllMocks()
  jest.mocked(useAddMeal).mockReturnValue(mockAddMeal)
  jest.mocked(useDeleteMeal).mockReturnValue(mockDeleteMeal)
  jest.mocked(useMoveMeal).mockReturnValue(mockMoveMeal)
  mockAddMeal.mockResolvedValue({})
  mockDeleteMeal.mockResolvedValue({})
  mockMoveMeal.mockResolvedValue({})
  mockOnPrevWeek.mockReset()
  mockOnNextWeek.mockReset()
})

function openAddDialog() {
  fireEvent.click(screen.getAllByRole('button', { name: '+' })[0])
}

// The add dialog now opens on the Recipe tab by default; switch to Custom for
// the free-text entry tests.
function openAddDialogCustom() {
  openAddDialog()
  fireEvent.click(screen.getByRole('button', { name: 'Custom' }))
}

function openMealMenu() {
  fireEvent.click(screen.getAllByRole('button', { name: /Meal actions/i })[0])
}

function startSwap() {
  openMealMenu()
  fireEvent.click(screen.getAllByRole('menuitem', { name: /Swap/i })[0])
}

describe('MealPlanCalendar', () => {
  it('shows slot label in grid', () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    // Both mobile and desktop views render slot labels
    expect(screen.getAllByText('Breakfast').length).toBeGreaterThan(0)
  })

  it('opens add dialog with Recipe and Custom tabs when + is clicked', () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialog()
    expect(screen.getByRole('button', { name: 'Recipe' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Custom' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Event' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Custom' }))
    expect(screen.getByPlaceholderText('Item 1')).toBeInTheDocument()
  })

  it('adds a custom item kept off the shopping list', async () => {
    const onMutate = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )
    openAddDialogCustom()
    fireEvent.change(screen.getByPlaceholderText('Item 1'), {
      target: { value: 'Birthday dinner' }
    })
    fireEvent.click(screen.getByLabelText(/keep off the shopping list/i))
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    const req = mockAddMeal.mock.calls[0][0]
    expect(req.excludeFromShoppingList).toBe(true)
    expect(req.recipeId).toBe('')
    expect(req.customName).toBe('Birthday dinner')
    expect(req.servings).toBe(1)
    expect(onMutate).toHaveBeenCalled()
  })

  it('opens the edit dialog on the Custom tab pre-populated for an excluded meal', () => {
    const planWithExcluded = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eating out',
          servings: 1,
          excludeFromShoppingList: true
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithExcluded} recipes={baseRecipes} {...defaultNavProps} />)
    openMealMenu()
    fireEvent.click(screen.getAllByRole('menuitem', { name: /Edit/i })[0])
    const input = screen.getByPlaceholderText('Item 1')
    if (!(input instanceof HTMLInputElement)) throw new Error('expected input')
    expect(input.value).toBe('Eating out')
    const checkbox = screen.getByLabelText(/keep off the shopping list/i)
    if (!(checkbox instanceof HTMLInputElement)) throw new Error('expected checkbox')
    expect(checkbox.checked).toBe(true)
  })

  it('shows recipe combobox when Recipe tab is selected', () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Recipe' }))
    expect(screen.getByPlaceholderText(/recipe name or custom meal/i)).toBeInTheDocument()
  })

  it('adds meal with recipeId when recipe selected from combobox', async () => {
    const onMutate = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )
    openAddDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Recipe' }))
    const input = screen.getByPlaceholderText(/recipe name or custom meal/i)
    fireEvent.change(input, { target: { value: 'Pasta' } })
    fireEvent.mouseDown(screen.getByText('Pasta'))
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    const req = mockAddMeal.mock.calls[0][0]
    expect(req.recipeId).toBe('r1')
    expect(req.customName).toBe('')
    expect(req.mealSlot).toBe('breakfast')
    expect(req.servings).toBe(1)
    expect(onMutate).toHaveBeenCalled()
  })

  it('adds meal with customName for free-text entry', async () => {
    const onMutate = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )
    openAddDialogCustom()
    fireEvent.change(screen.getByPlaceholderText('Item 1'), {
      target: { value: 'Homemade soup' }
    })
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    const req = mockAddMeal.mock.calls[0][0]
    expect(req.recipeId).toBe('')
    expect(req.customName).toBe('Homemade soup')
    expect(req.servings).toBe(1)
    expect(onMutate).toHaveBeenCalled()
  })

  it('encodes a custom item amount into customName', async () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialogCustom()
    fireEvent.change(screen.getByPlaceholderText('Item 1'), {
      target: { value: 'Olive oil' }
    })
    fireEvent.change(screen.getByLabelText('Amount for item 1'), {
      target: { value: '2' }
    })
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    expect(mockAddMeal.mock.calls[0][0].customName).toBe('Olive oil\t2')
  })

  it('sends custom servings value in AddMealRequest', async () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Recipe' }))
    const input = screen.getByPlaceholderText(/recipe name or custom meal/i)
    fireEvent.change(input, { target: { value: 'Pasta' } })
    fireEvent.mouseDown(screen.getByText('Pasta'))
    fireEvent.change(screen.getByPlaceholderText('Servings'), {
      target: { value: '4' }
    })
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    expect(mockAddMeal.mock.calls[0][0].servings).toBe(4)
  })

  it('does not submit when custom item is empty', async () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialogCustom()
    // Item 1 is empty
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).not.toHaveBeenCalled())
  })

  it('submits recipe on Enter key in combobox after selecting', async () => {
    const onMutate = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )
    openAddDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Recipe' }))
    const input = screen.getByPlaceholderText(/recipe name or custom meal/i)
    fireEvent.change(input, { target: { value: 'Pasta' } })
    fireEvent.mouseDown(screen.getByText('Pasta'))
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
  })

  it('displays customName for meals without a matching recipe', () => {
    const planWithCustomMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs and toast',
          servings: 2
        })
      ]
    }

    render(
      <MealPlanCalendar plan={planWithCustomMeal} recipes={baseRecipes} {...defaultNavProps} />
    )
    expect(screen.getAllByText(/Eggs and toast/)[0]).toBeInTheDocument()
  })

  it('shows servings multiplier badge when servings > 1', () => {
    const planWithServings = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: 'r1',
          customName: '',
          servings: 3
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithServings} recipes={baseRecipes} {...defaultNavProps} />)
    expect(screen.getAllByText('×3')[0]).toBeInTheDocument()
  })

  it('does not show servings badge when servings is 1', () => {
    const planWithSingleServing = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: 'r1',
          customName: '',
          servings: 1
        })
      ]
    }

    render(
      <MealPlanCalendar plan={planWithSingleServing} recipes={baseRecipes} {...defaultNavProps} />
    )
    expect(screen.queryByText('×1')).not.toBeInTheDocument()
  })

  it('cancel closes the add dialog', () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialogCustom()
    expect(screen.getByPlaceholderText('Item 1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^Cancel$/i }))
    expect(screen.queryByPlaceholderText('Item 1')).not.toBeInTheDocument()
  })

  it('shows swap banner when a meal is selected', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)

    startSwap()
    expect(screen.getByText(/Swapping/i)).toBeInTheDocument()
  })

  it('calls moveMeal and onMutate when placing a selected meal on an empty cell', async () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    const onMutate = jest.fn()
    render(
      <MealPlanCalendar
        plan={planWithMeal}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )

    // Select the meal for swapping via its actions menu
    startSwap()
    // In swap mode the "+" buttons are hidden; click the cell div directly.
    // All cells have hover:border-accent class in swap mode; index 1 is the first empty slot.
    const swapCells = document.querySelectorAll('[class*="hover:border-accent"]')
    fireEvent.click(swapCells[1])

    await waitFor(() => expect(mockMoveMeal).toHaveBeenCalled())
    // Empty target → only the picked meal moves, no swap-back.
    expect(mockMoveMeal).toHaveBeenCalledTimes(1)
    const req = mockMoveMeal.mock.calls[0][0]
    expect(req.mealId).toBe('m1')
    expect(req.newSlot).toBe('breakfast')
    await waitFor(() => expect(onMutate).toHaveBeenCalled())
  })

  it('swaps two meals positions when placing onto an occupied cell', async () => {
    const planWithMeals = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        }),
        makePlanMeal({
          id: 'm2',
          mealDate: '2026-05-26',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Pancakes',
          servings: 1
        })
      ]
    }

    const onMutate = jest.fn()
    render(
      <MealPlanCalendar
        plan={planWithMeals}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )

    // Pick m1 (Eggs) for swapping, then tap m2 (Pancakes) to swap positions.
    startSwap()
    const target = screen
      .getAllByText(/Pancakes/)
      .find((el) => el.classList.contains('wrap-break-word'))!
    fireEvent.click(target)

    await waitFor(() => expect(mockMoveMeal).toHaveBeenCalledTimes(2))
    const first = mockMoveMeal.mock.calls[0][0]
    const second = mockMoveMeal.mock.calls[1][0]
    // Picked meal moves to the target's slot.
    expect(first.mealId).toBe('m1')
    expect(first.newDate).toBe('2026-05-26')
    // Target meal moves back to the picked meal's original slot.
    expect(second.mealId).toBe('m2')
    expect(second.newDate).toBe('2026-05-25')
    await waitFor(() => expect(onMutate).toHaveBeenCalled())
  })

  it('deselects meal when clicking it again', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)

    // Start a swap via the actions menu
    startSwap()
    expect(screen.getByText(/Swapping/i)).toBeInTheDocument()

    // In swap mode, clicking the same chip body cancels. The item has the 'wrap-break-word' class.
    const mealItem = screen
      .getAllByText(/Eggs/)
      .find((el) => el.classList.contains('wrap-break-word'))!
    fireEvent.click(mealItem)
    expect(screen.queryByText(/Swapping/i)).not.toBeInTheDocument()
  })

  it('calls onPrevWeek when Previous Week button is clicked', () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    fireEvent.click(screen.getByRole('button', { name: /Prev/i }))
    expect(mockOnPrevWeek).toHaveBeenCalledTimes(1)
  })

  it('calls onNextWeek when Next Week button is clicked', () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    fireEvent.click(screen.getByRole('button', { name: /Next/i }))
    expect(mockOnNextWeek).toHaveBeenCalledTimes(1)
  })

  it('shows week date range in dd/mm/yyyy format', () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    // Mock week: 2026-05-25 to 2026-05-31
    expect(screen.getByText('25/05/2026 – 31/05/2026')).toBeInTheDocument()
  })

  it('shows edit button on meal card', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)
    openMealMenu()
    expect(screen.getAllByRole('menuitem', { name: /Edit/i })[0]).toBeInTheDocument()
  })

  it('clicking edit button opens edit dialog pre-populated with custom items', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 3
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)
    openMealMenu()
    fireEvent.click(screen.getAllByRole('menuitem', { name: /Edit/i })[0])
    expect(screen.getByText(/Edit meal/i)).toBeInTheDocument()
    const input = screen.getByPlaceholderText('Item 1')
    if (!(input instanceof HTMLInputElement)) throw new Error('expected input')
    expect(input.value).toBe('Eggs')
  })

  it('save edit calls addMeal with same date/slot and new values, then onMutate', async () => {
    const onMutate = jest.fn()
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(
      <MealPlanCalendar
        plan={planWithMeal}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )
    openMealMenu()
    fireEvent.click(screen.getAllByRole('menuitem', { name: /Edit/i })[0])
    const input = screen.getByPlaceholderText('Item 1')
    fireEvent.change(input, { target: { value: 'Updated meal' } })
    fireEvent.click(screen.getByRole('button', { name: /^Save$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    const req = mockAddMeal.mock.calls[0][0]
    expect(req.mealDate).toBe('2026-05-25')
    expect(req.mealSlot).toBe('breakfast')
    expect(req.customName).toBe('Updated meal')
    expect(onMutate).toHaveBeenCalled()
  })

  it('cancel closes edit dialog', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)
    openMealMenu()
    fireEvent.click(screen.getAllByRole('menuitem', { name: /Edit/i })[0])
    expect(screen.getByText(/Edit meal/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^Cancel$/i }))
    expect(screen.queryByRole('button', { name: /^Save$/i })).not.toBeInTheDocument()
  })

  it('actions menu trigger is hidden during swap mode', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)
    // Enter swap mode via the actions menu; the trigger then disappears.
    startSwap()
    expect(screen.queryAllByRole('button', { name: /Meal actions/i })).toHaveLength(0)
  })

  it('highlights today with accent label in mobile view', () => {
    // Pin the clock so "today" falls within the mocked week (2026-05-25–2026-05-31).
    jest.useFakeTimers()
    jest.setSystemTime(new Date('2026-05-30'))
    try {
      render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
      // Mobile view appends "(today)" next to the day header for today.
      expect(screen.getAllByText('(today)').length).toBeGreaterThan(0)
    } finally {
      jest.useRealTimers()
    }
  })

  it('cancels swap when Cancel button in banner is clicked', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)

    startSwap()
    expect(screen.getByText(/Swapping/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Cancel/i }))
    expect(screen.queryByText(/Swapping/i)).not.toBeInTheDocument()
  })

  it('does not show add button when slot already has a meal', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMeal} recipes={baseRecipes} {...defaultNavProps} />)
    // The occupied slot (2026-05-25 breakfast) should not have a "+" button;
    // only the remaining 6 empty days have them.
    const addButtons = screen.getAllByRole('button', { name: '+' })
    // 7 days × 1 slot, but 1 slot is occupied → 6 "+" buttons per view (mobile + desktop = 12)
    expect(addButtons.length).toBeLessThan(14)
  })

  it('adds multiple custom items joined by newline', async () => {
    const onMutate = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onMutate={onMutate}
      />
    )
    openAddDialogCustom()
    // Fill first item
    fireEvent.change(screen.getByPlaceholderText('Item 1'), {
      target: { value: 'Chicken' }
    })
    // Add second item
    fireEvent.click(screen.getByRole('button', { name: /\+ Add item/i }))
    fireEvent.change(screen.getByPlaceholderText('Item 2'), {
      target: { value: 'Rice' }
    })
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    const req = mockAddMeal.mock.calls[0][0]
    expect(req.customName).toBe('Chicken\nRice')
    expect(req.recipeId).toBe('')
  })

  it('adds a new item row on Enter in the amount field and removes a row', async () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialogCustom()
    fireEvent.change(screen.getByPlaceholderText('Item 1'), {
      target: { value: 'Chicken' }
    })
    // Enter in the amount field adds a second row.
    fireEvent.keyDown(screen.getByLabelText('Amount for item 1'), { key: 'Enter' })
    expect(screen.getByPlaceholderText('Item 2')).toBeInTheDocument()
    // Removing the second row collapses back to one.
    const removeButtons = screen.getAllByRole('button', { name: /Remove item/i })
    fireEvent.click(removeButtons[removeButtons.length - 1])
    expect(screen.queryByPlaceholderText('Item 2')).not.toBeInTheDocument()
  })

  it('displays multiple custom items as bullet list in meal chip', () => {
    const planWithMultiItem = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Chicken\nRice',
          servings: 1
        })
      ]
    }

    render(<MealPlanCalendar plan={planWithMultiItem} recipes={baseRecipes} {...defaultNavProps} />)
    expect(screen.getAllByText(/Chicken/)[0]).toBeInTheDocument()
    expect(screen.getAllByText(/Rice/)[0]).toBeInTheDocument()
  })

  it('sends a single addMeal request when adding a custom item (no double add)', async () => {
    render(<MealPlanCalendar plan={basePlan} recipes={baseRecipes} {...defaultNavProps} />)
    openAddDialogCustom()
    fireEvent.change(screen.getByPlaceholderText('Item 1'), {
      target: { value: 'Homemade soup' }
    })
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    expect(mockAddMeal).toHaveBeenCalledTimes(1)
  })

  it('renders a single chip when a slot somehow holds more than one meal', () => {
    const planWithDuplicates = {
      ...basePlan,
      meals: [
        makePlanMeal({
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        }),
        makePlanMeal({
          id: 'm2',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Pancakes',
          servings: 1
        })
      ]
    }

    render(
      <MealPlanCalendar plan={planWithDuplicates} recipes={baseRecipes} {...defaultNavProps} />
    )
    // Only the first meal is shown; the duplicate never renders a second chip.
    expect(screen.getAllByText(/Eggs/).length).toBeGreaterThan(0)
    expect(screen.queryByText(/Pancakes/)).not.toBeInTheDocument()
    // One chip per view (mobile + desktop) → two action triggers, not four.
    expect(screen.getAllByRole('button', { name: /Meal actions/i })).toHaveLength(2)
  })
})
