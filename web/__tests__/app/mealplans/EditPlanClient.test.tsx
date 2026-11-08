import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/hooks/useMealPlans', () => ({
  useMealPlan: vi.fn()
}))

vi.mock('@/hooks/navigation', () => ({
  useRouter: () => ({ push: vi.fn() })
}))

vi.mock('@/components/mealplans/PlanForm', () => ({
  __esModule: true,
  default: () => <div data-testid="plan-form" />
}))

import EditPlanClient from '@/app/mealplans/[id]/edit/EditPlanClient'
import { useMealPlan } from '@/hooks/useMealPlans'
import { create } from '@bufbuild/protobuf'
import { PlanSchema, GetPlanResponseSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { GetPlanResponse } from '@/lib/gen/mealplans/v1/mealplans_pb'

function mockPlan(value: { data?: GetPlanResponse; error?: Error; isLoading: boolean }) {
  vi.mocked(useMealPlan).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: vi.fn(async () => undefined)
  })
}

beforeEach(() => vi.clearAllMocks())

describe('EditPlanClient', () => {
  it('shows a loading state', () => {
    mockPlan({ isLoading: true })
    render(<EditPlanClient id="plan-1" />)
    expect(screen.getByText('Loading plan…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockPlan({ error: new Error('boom'), isLoading: false })
    render(<EditPlanClient id="plan-1" />)
    expect(screen.getByText('Failed to load plan.')).toBeInTheDocument()
  })

  it('renders the plan form once loaded', () => {
    mockPlan({
      data: create(GetPlanResponseSchema, {
        plan: create(PlanSchema, { id: 'plan-1', name: 'My Plan' })
      }),
      isLoading: false
    })
    render(<EditPlanClient id="plan-1" />)
    expect(screen.getByTestId('plan-form')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Edit plan' })).toBeInTheDocument()
  })
})
