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
import type { Plan } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

const mockAddMeal = jest.fn()
const mockDeleteMeal = jest.fn()
const mockMoveMeal = jest.fn()

const basePlan = { id: 'plan-1', name: 'Test Plan', meals: [] } as unknown as Plan
const baseRecipes = [{ id: 'r1', name: 'Pasta' }] as unknown as Recipe[]

const mockOnPrevWeek = jest.fn()
const mockOnNextWeek = jest.fn()
const defaultNavProps = {
  weekOffset: 0,
  onPrevWeek: mockOnPrevWeek,
  onNextWeek: mockOnNextWeek
}

beforeEach(() => {
  jest.clearAllMocks()
  ;(useAddMeal as jest.Mock).mockReturnValue(mockAddMeal)
  ;(useDeleteMeal as jest.Mock).mockReturnValue(mockDeleteMeal)
  ;(useMoveMeal as jest.Mock).mockReturnValue(mockMoveMeal)
  mockAddMeal.mockResolvedValue({})
  mockDeleteMeal.mockResolvedValue({})
  mockMoveMeal.mockResolvedValue({})
  mockOnPrevWeek.mockReset()
  mockOnNextWeek.mockReset()
})

function openAddPanel() {
  fireEvent.click(screen.getAllByRole('button', { name: '+' })[0])
}

describe('MealPlanCalendar', () => {
  it('shows slot label in grid', () => {
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    expect(screen.getByText('Breakfast')).toBeInTheDocument() // capitalized for display
  })

  it('renders combobox input when add panel opens', () => {
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    openAddPanel()
    expect(screen.getByPlaceholderText(/recipe name or custom meal/i)).toBeInTheDocument()
  })

  it('adds meal with recipeId when recipe selected from combobox', async () => {
    const onAddMeal = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={onAddMeal}
        onDeleteMeal={jest.fn()}
      />
    )
    openAddPanel()
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
    expect(onAddMeal).toHaveBeenCalled()
  })

  it('adds meal with customName for free-text entry', async () => {
    const onAddMeal = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={onAddMeal}
        onDeleteMeal={jest.fn()}
      />
    )
    openAddPanel()
    fireEvent.change(screen.getByPlaceholderText(/recipe name or custom meal/i), {
      target: { value: 'Homemade soup' }
    })
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    const req = mockAddMeal.mock.calls[0][0]
    expect(req.recipeId).toBe('')
    expect(req.customName).toBe('Homemade soup')
    expect(req.servings).toBe(1)
    expect(onAddMeal).toHaveBeenCalled()
  })

  it('sends custom servings value in AddMealRequest', async () => {
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    openAddPanel()
    fireEvent.change(screen.getByPlaceholderText(/recipe name or custom meal/i), {
      target: { value: 'Salad' }
    })
    fireEvent.change(screen.getByPlaceholderText('Servings'), {
      target: { value: '4' }
    })
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
    expect(mockAddMeal.mock.calls[0][0].servings).toBe(4)
  })

  it('does not submit when combobox is empty', async () => {
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    openAddPanel()
    fireEvent.click(screen.getByRole('button', { name: /^Add$/i }))
    await waitFor(() => expect(mockAddMeal).not.toHaveBeenCalled())
  })

  it('submits on Enter key in combobox', async () => {
    const onAddMeal = jest.fn()
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={onAddMeal}
        onDeleteMeal={jest.fn()}
      />
    )
    openAddPanel()
    const input = screen.getByPlaceholderText(/recipe name or custom meal/i)
    fireEvent.change(input, { target: { value: 'Salad' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(mockAddMeal).toHaveBeenCalled())
  })

  it('displays customName for meals without a matching recipe', () => {
    const planWithCustomMeal = {
      ...basePlan,
      meals: [
        {
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs and toast',
          servings: 2
        }
      ]
    } as unknown as Plan

    render(
      <MealPlanCalendar
        plan={planWithCustomMeal}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    expect(screen.getByText('Eggs and toast')).toBeInTheDocument()
  })

  it('shows servings multiplier badge when servings > 1', () => {
    const planWithServings = {
      ...basePlan,
      meals: [
        {
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: 'r1',
          customName: '',
          servings: 3
        }
      ]
    } as unknown as Plan

    render(
      <MealPlanCalendar
        plan={planWithServings}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    expect(screen.getByText('×3')).toBeInTheDocument()
  })

  it('does not show servings badge when servings is 1', () => {
    const planWithSingleServing = {
      ...basePlan,
      meals: [
        {
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: 'r1',
          customName: '',
          servings: 1
        }
      ]
    } as unknown as Plan

    render(
      <MealPlanCalendar
        plan={planWithSingleServing}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    expect(screen.queryByText('×1')).not.toBeInTheDocument()
  })

  it('cancel closes the add panel', () => {
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    openAddPanel()
    expect(screen.getByPlaceholderText(/recipe name or custom meal/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Cancel/i }))
    expect(screen.queryByPlaceholderText(/recipe name or custom meal/i)).not.toBeInTheDocument()
  })

  it('shows move banner when a meal is selected', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        {
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        }
      ]
    } as unknown as Plan

    render(
      <MealPlanCalendar
        plan={planWithMeal}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )

    fireEvent.click(screen.getByText('Eggs'))
    expect(screen.getByText(/Moving/i)).toBeInTheDocument()
  })

  it('calls moveMeal and onMoveMeal when placing a selected meal on an empty cell', async () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        {
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        }
      ]
    } as unknown as Plan

    const onMoveMeal = jest.fn()
    render(
      <MealPlanCalendar
        plan={planWithMeal}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
        onMoveMeal={onMoveMeal}
      />
    )

    // Select the meal
    fireEvent.click(screen.getByText('Eggs'))
    // Click an empty cell (second day = 2026-05-26)
    const cells = document.querySelectorAll('[class*="border rounded"]')
    fireEvent.click(cells[1])

    await waitFor(() => expect(mockMoveMeal).toHaveBeenCalled())
    const req = mockMoveMeal.mock.calls[0][0]
    expect(req.mealId).toBe('m1')
    expect(req.newSlot).toBe('breakfast')
    await waitFor(() => expect(onMoveMeal).toHaveBeenCalled())
  })

  it('deselects meal when clicking it again', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        {
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        }
      ]
    } as unknown as Plan

    render(
      <MealPlanCalendar
        plan={planWithMeal}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )

    // First click selects the meal (only one "Eggs" present — the span in the card)
    fireEvent.click(screen.getByText('Eggs'))
    expect(screen.getByText(/Moving/i)).toBeInTheDocument()

    // After selection the banner shows a <strong>Eggs</strong> too; click the span in the card
    const mealSpan = screen.getAllByText('Eggs').find((el) => el.classList.contains('truncate'))!
    fireEvent.click(mealSpan)
    expect(screen.queryByText(/Moving/i)).not.toBeInTheDocument()
  })

  it('calls onPrevWeek when Previous Week button is clicked', () => {
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: /Previous Week/i }))
    expect(mockOnPrevWeek).toHaveBeenCalledTimes(1)
  })

  it('calls onNextWeek when Next Week button is clicked', () => {
    render(
      <MealPlanCalendar
        plan={basePlan}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: /Next Week/i }))
    expect(mockOnNextWeek).toHaveBeenCalledTimes(1)
  })

  it('cancels move when Cancel button in banner is clicked', () => {
    const planWithMeal = {
      ...basePlan,
      meals: [
        {
          id: 'm1',
          mealDate: '2026-05-25',
          mealSlot: 'breakfast',
          recipeId: '',
          customName: 'Eggs',
          servings: 1
        }
      ]
    } as unknown as Plan

    render(
      <MealPlanCalendar
        plan={planWithMeal}
        recipes={baseRecipes}
        {...defaultNavProps}
        onAddMeal={jest.fn()}
        onDeleteMeal={jest.fn()}
      />
    )

    fireEvent.click(screen.getByText('Eggs'))
    expect(screen.getByText(/Moving/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Cancel/i }))
    expect(screen.queryByText(/Moving/i)).not.toBeInTheDocument()
  })
})
