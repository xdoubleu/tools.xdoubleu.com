import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useMealPlans', () => ({
  useMealPlan: jest.fn()
}))

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() })
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/recipes/PlanForm', () => ({
  __esModule: true,
  default: () => <div data-testid="plan-form" />
}))

import EditPlanClient from '@/app/mealplans/[id]/edit/EditPlanClient'
import { useMealPlan } from '@/hooks/useMealPlans'
import { create } from '@bufbuild/protobuf'
import { PlanSchema, GetPlanResponseSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { GetPlanResponse } from '@/lib/gen/mealplans/v1/mealplans_pb'

function mockPlan(value: { data?: GetPlanResponse; error?: Error; isLoading: boolean }) {
  jest.mocked(useMealPlan).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: jest.fn(async () => undefined)
  })
}

beforeEach(() => jest.clearAllMocks())

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
      data: create(GetPlanResponseSchema, { plan: create(PlanSchema, { id: 'plan-1', name: 'My Plan' }) }),
      isLoading: false
    })
    render(<EditPlanClient id="plan-1" />)
    expect(screen.getByTestId('plan-form')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument()
  })
})
