import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'

jest.mock('@/hooks/useMealPlans', () => ({
  useMealPlan: jest.fn(),
  useAddMeal: jest.fn(),
  useDeleteMeal: jest.fn(),
  useSharePlan: jest.fn(),
  useUnsharePlan: jest.fn(),
  useDeletePlan: jest.fn()
}))

jest.mock('@/hooks/useRecipes', () => ({
  useRecipes: jest.fn()
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

jest.mock('@/lib/env', () => ({ getApiUrl: () => 'http://localhost' }))

import MealPlanClient from '@/app/mealplans/[id]/MealPlanClient'
import { useMealPlan } from '@/hooks/useMealPlans'
import { useRecipes } from '@/hooks/useRecipes'
import { useRouter } from 'next/navigation'
import { create } from '@bufbuild/protobuf'
import { PlanSchema, GetPlanResponseSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import { RecipeSchema, ListRecipesResponseSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: jest.fn() }
const mockPlan = create(PlanSchema, {
  id: 'plan-1',
  name: 'Test Plan',
  canEdit: true
})
const mockRecipes = [
  create(RecipeSchema, { id: 'r1', name: 'Pasta' }),
  create(RecipeSchema, { id: 'r2', name: 'Salad' })
]

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- mock router returns partial AppRouterInstance
  jest.mocked(useRouter).mockReturnValue(mockRouter)
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  jest.mocked(useMealPlan).mockReturnValue({
    data: create(GetPlanResponseSchema, {
      plan: mockPlan,
      isOwner: true,
      icalUrl: 'http://example.com/ical',
      windowStart: '2026-05-25',
      windowEnd: '2026-05-31'
    }),
    error: undefined,
    isLoading: false
  })
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  jest.mocked(useRecipes).mockReturnValue({
    data: create(ListRecipesResponseSchema, { recipes: mockRecipes }),
    error: undefined,
    isLoading: false
  })
})

describe('MealPlanClient', () => {
  it('renders the plan name', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByText('Test Plan')).toBeInTheDocument()
  })

  it('does not render a shopping list toggle button', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.queryByRole('button', { name: /Shopping List/i })).not.toBeInTheDocument()
  })

  it('renders meal plan calendar', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByTestId('meal-plan-calendar')).toBeInTheDocument()
  })

  it('passes all recipes from useRecipes to MealPlanCalendar', async () => {
    render(<MealPlanClient id="plan-1" />)
    await waitFor(() => {
      expect(useRecipes).toHaveBeenCalled()
    })
  })

  it('shows iCal URL when available', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByText('http://example.com/ical')).toBeInTheDocument()
  })
})
