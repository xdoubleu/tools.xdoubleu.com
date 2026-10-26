import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'

vi.mock('@/hooks/useMealPlans', () => ({
  useMealPlans: vi.fn()
}))

const replace = vi.fn()
vi.mock('@/hooks/navigation', () => ({
  useRouter: () => ({ replace })
}))

import PlansListClient from '@/components/mealplans/PlansListClient'
import { useMealPlans } from '@/hooks/useMealPlans'
import { create } from '@bufbuild/protobuf'
import { PlanSchema, ListPlansResponseSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'
import type { ListPlansResponse } from '@/lib/gen/mealplans/v1/mealplans_pb'

const mutate = vi.fn(async () => undefined)

function mockUseMealPlans(value: { data?: ListPlansResponse; error?: Error; isLoading: boolean }) {
  vi.mocked(useMealPlans).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate
  })
}

beforeEach(() => vi.clearAllMocks())

describe('PlansListClient', () => {
  it('shows a loading state', () => {
    mockUseMealPlans({ isLoading: true })
    render(<PlansListClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockUseMealPlans({ error: new Error('boom'), isLoading: false })
    render(<PlansListClient />)
    expect(screen.getByText('Failed to load meal plan.')).toBeInTheDocument()
  })

  it('re-lists, which creates the first plan, when there are no plans', () => {
    mockUseMealPlans({ data: create(ListPlansResponseSchema, { plans: [] }), isLoading: false })
    render(<PlansListClient />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Create Meal Plan' }))
    expect(mutate).toHaveBeenCalled()
    expect(replace).not.toHaveBeenCalled()
  })

  it('redirects to the first plan when plans exist', () => {
    mockUseMealPlans({
      data: create(ListPlansResponseSchema, { plans: [create(PlanSchema, { id: 'plan-1' })] }),
      isLoading: false
    })
    render(<PlansListClient />)
    expect(replace).toHaveBeenCalledWith('/mealplans/plan-1')
  })
})
