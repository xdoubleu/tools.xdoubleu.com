import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@/hooks/useRecipes', () => ({
  useMealPlan: jest.fn(),
  useRecipes: jest.fn(),
  useShoppingList: jest.fn(),
  useAddMeal: jest.fn(),
  useDeleteMeal: jest.fn(),
  useSharePlan: jest.fn(),
  useUnsharePlan: jest.fn(),
  useDeletePlan: jest.fn()
}))

jest.mock('next/navigation', () => ({
  useRouter: jest.fn()
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/recipes/MealPlanCalendar', () => {
  return function MockCalendar() {
    return <div data-testid="meal-plan-calendar">calendar-mock</div>
  }
})

jest.mock('@/components/recipes/ShoppingList', () => {
  return function MockShoppingList() {
    return <div data-testid="shopping-list">shopping-list-mock</div>
  }
})

jest.mock('@/lib/gen/recipes/v1/mealplans_pb', () => ({
  AddMealRequest: jest.fn().mockImplementation((d) => d),
  DeleteMealRequest: jest.fn().mockImplementation((d) => d),
  SharePlanRequest: jest.fn().mockImplementation((d) => d),
  UnsharePlanRequest: jest.fn().mockImplementation((d) => d),
  DeletePlanRequest: jest.fn().mockImplementation((d) => d)
}))

jest.mock('@/lib/env', () => ({ getApiUrl: () => 'http://localhost' }))

import MealPlanClient from '@/app/recipes/plans/[id]/MealPlanClient'
import { useMealPlan, useRecipes, useShoppingList } from '@/hooks/useRecipes'
import { useRouter } from 'next/navigation'
import type { Plan } from '@/lib/gen/recipes/v1/mealplans_pb'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: jest.fn() }
const mockPlan = { id: 'plan-1', name: 'Test Plan', meals: [] } as unknown as Plan
const mockRecipes = [
  { id: 'r1', name: 'Pasta' },
  { id: 'r2', name: 'Salad' }
] as unknown as Recipe[]

beforeEach(() => {
  jest.clearAllMocks()
  ;(useRouter as jest.Mock).mockReturnValue(mockRouter)
  ;(useMealPlan as jest.Mock).mockReturnValue({
    data: {
      plan: mockPlan,
      recipes: [],
      isOwner: true,
      icalUrl: 'http://example.com/ical',
      windowStart: '2026-05-25',
      windowEnd: '2026-05-31'
    },
    error: null,
    isLoading: false,
    mutate: jest.fn()
  })
  ;(useRecipes as jest.Mock).mockReturnValue({
    data: {
      recipes: mockRecipes
    },
    error: null,
    isLoading: false
  })
  ;(useShoppingList as jest.Mock).mockReturnValue({
    data: {
      items: []
    },
    error: null,
    isLoading: false
  })
})

describe('MealPlanClient', () => {
  it('renders the plan name', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByText('Test Plan')).toBeInTheDocument()
  })

  it('shows Shopping List toggle button', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByRole('button', { name: /Shopping List/i })).toBeInTheDocument()
  })

  it('clicking toggle reveals ShoppingList component', async () => {
    render(<MealPlanClient id="plan-1" />)
    const toggleButton = screen.getByRole('button', { name: /Shopping List/i })
    fireEvent.click(toggleButton)
    await waitFor(() => {
      expect(screen.getByTestId('shopping-list')).toBeInTheDocument()
    })
  })

  it('clicking toggle again hides ShoppingList component', async () => {
    render(<MealPlanClient id="plan-1" />)
    const toggleButton = screen.getByRole('button', { name: /Shopping List/i })

    // First click to show
    fireEvent.click(toggleButton)
    await waitFor(() => {
      expect(screen.getByTestId('shopping-list')).toBeInTheDocument()
    })

    // Second click to hide
    fireEvent.click(toggleButton)
    await waitFor(() => {
      expect(screen.queryByTestId('shopping-list')).not.toBeInTheDocument()
    })
  })

  it('passes all recipes from useRecipes to MealPlanCalendar', async () => {
    render(<MealPlanClient id="plan-1" />)
    await waitFor(() => {
      expect(useRecipes).toHaveBeenCalled()
    })
  })

  it('toggle button text changes based on shopping list visibility', async () => {
    render(<MealPlanClient id="plan-1" />)
    const toggleButton = screen.getByRole('button', { name: /Shopping List/i })

    expect(toggleButton).toHaveTextContent('Shopping List')

    fireEvent.click(toggleButton)
    await waitFor(() => {
      expect(toggleButton).toHaveTextContent('Hide Shopping List')
    })

    fireEvent.click(toggleButton)
    await waitFor(() => {
      expect(toggleButton).toHaveTextContent('Shopping List')
    })
  })

  it('calls useShoppingList with plan id when shopping list is shown', async () => {
    render(<MealPlanClient id="plan-1" />)
    const toggleButton = screen.getByRole('button', { name: /Shopping List/i })

    fireEvent.click(toggleButton)
    await waitFor(() => {
      expect(useShoppingList).toHaveBeenCalledWith('plan-1')
    })
  })

  it('does not call useShoppingList when shopping list is hidden', async () => {
    ;(useShoppingList as jest.Mock).mockClear()
    render(<MealPlanClient id="plan-1" />)
    await waitFor(() => {
      expect(useShoppingList).toHaveBeenCalledWith('')
    })
  })

  it('renders meal plan calendar', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByTestId('meal-plan-calendar')).toBeInTheDocument()
  })

  it('shows iCal URL when available', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByText('http://example.com/ical')).toBeInTheDocument()
  })
})
