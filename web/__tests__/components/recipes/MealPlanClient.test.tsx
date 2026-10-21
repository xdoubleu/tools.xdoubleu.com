import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'

vi.mock('@/hooks/useMealPlans', () => ({
  useMealPlan: vi.fn(),
  useAddMeal: vi.fn(),
  useDeleteMeal: vi.fn(),
  useSharePlan: vi.fn(),
  useUnsharePlan: vi.fn()
}))

vi.mock('@/hooks/useRecipes', () => ({
  useAllRecipes: vi.fn()
}))

vi.mock('@/hooks/navigation', () => ({
  useRouter: vi.fn()
}))

vi.mock('@/components/mealplans/MealPlanCalendar', () => {
  return {
    default: function MockCalendar() {
      return <div data-testid="meal-plan-calendar">calendar-mock</div>
    }
  }
})

vi.mock('@/lib/env', () => ({ getApiUrl: () => 'http://localhost' }))

import MealPlanClient from '@/app/mealplans/[id]/MealPlanClient'
import { useMealPlan } from '@/hooks/useMealPlans'
import { useAllRecipes } from '@/hooks/useRecipes'
import { useRouter } from '@/hooks/navigation'
import { create } from '@bufbuild/protobuf'
import { PlanSchema, GetPlanResponseSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import { RecipeSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: vi.fn() }
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
  vi.clearAllMocks()
  // @ts-expect-error -- mock router returns partial AppRouterInstance
  vi.mocked(useRouter).mockReturnValue(mockRouter)
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  vi.mocked(useMealPlan).mockReturnValue({
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
  vi.mocked(useAllRecipes).mockReturnValue({
    data: mockRecipes,
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

  it('passes all recipes from useAllRecipes to MealPlanCalendar', async () => {
    render(<MealPlanClient id="plan-1" />)
    await waitFor(() => {
      expect(useAllRecipes).toHaveBeenCalled()
    })
  })

  it('shows iCal Link button when icalUrl is available', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByRole('button', { name: 'iCal Link' })).toBeInTheDocument()
  })

  it('shows Settings link instead of Edit for owner', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Edit' })).not.toBeInTheDocument()
  })

  it('does not show a Delete button', () => {
    render(<MealPlanClient id="plan-1" />)
    expect(screen.queryByRole('button', { name: /delete/i })).not.toBeInTheDocument()
  })
})
