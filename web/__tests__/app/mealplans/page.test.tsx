import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useMealPlans', () => ({
  useMealPlans: jest.fn()
}))

const replace = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace })
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

import PlansPage from '@/app/mealplans/page'
import { useMealPlans } from '@/hooks/useMealPlans'
import { create } from '@bufbuild/protobuf'
import { PlanSchema, ListPlansResponseSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { ListPlansResponse } from '@/lib/gen/mealplans/v1/mealplans_pb'

function mockUseMealPlans(value: { data?: ListPlansResponse; error?: Error; isLoading: boolean }) {
  jest.mocked(useMealPlans).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: jest.fn(async () => undefined)
  })
}

beforeEach(() => jest.clearAllMocks())

describe('PlansPage', () => {
  it('shows a loading state', () => {
    mockUseMealPlans({ isLoading: true })
    render(<PlansPage />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockUseMealPlans({ error: new Error('boom'), isLoading: false })
    render(<PlansPage />)
    expect(screen.getByText('Failed to load meal plan.')).toBeInTheDocument()
  })

  it('shows a Create Meal Plan link when there are no plans', () => {
    mockUseMealPlans({ data: create(ListPlansResponseSchema, { plans: [] }), isLoading: false })
    render(<PlansPage />)
    const link = screen.getByRole('link', { name: 'Create Meal Plan' })
    expect(link).toHaveAttribute('href', '/mealplans/new')
    expect(replace).not.toHaveBeenCalled()
  })

  it('redirects to the first plan when plans exist', () => {
    mockUseMealPlans({
      data: create(ListPlansResponseSchema, { plans: [create(PlanSchema, { id: 'plan-1' })] }),
      isLoading: false
    })
    render(<PlansPage />)
    expect(replace).toHaveBeenCalledWith('/mealplans/plan-1')
  })
})
