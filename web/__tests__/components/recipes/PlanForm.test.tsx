import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@/hooks/useMealPlans', () => ({
  useUpdatePlan: jest.fn()
}))
jest.mock('@/lib/gen/mealplans/v1/mealplans_pb', () => ({
  ...jest.requireActual('@/lib/gen/mealplans/v1/mealplans_pb'),
  UpdatePlanRequest: jest.fn().mockImplementation((d) => d)
}))

import { useUpdatePlan } from '@/hooks/useMealPlans'
import PlanForm from '@/components/recipes/PlanForm'
import { create } from '@bufbuild/protobuf'
import { PlanSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'

const mockUpdate = jest.fn()

const existingPlan = create(PlanSchema, {
  id: 'plan-1',
  name: 'Existing Plan',
  canEdit: true,
  icalHideSlots: ['breakfast'],
  icalHidePast: true
})

beforeEach(() => {
  jest.clearAllMocks()
  jest.mocked(useUpdatePlan).mockReturnValue(mockUpdate)
  mockUpdate.mockResolvedValue({})
})

describe('PlanForm', () => {
  it('renders name input and iCal controls', () => {
    render(<PlanForm plan={existingPlan} onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByLabelText(/Plan Name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Breakfast/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Noon/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Evening/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Hide past events/i)).toBeInTheDocument()
  })

  it('pre-fills name from existing plan', () => {
    render(<PlanForm plan={existingPlan} onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByLabelText(/Plan Name/i)).toHaveValue('Existing Plan')
  })

  it('pre-checks hidden slots', () => {
    render(<PlanForm plan={existingPlan} onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByLabelText(/Breakfast/i)).toBeChecked()
    expect(screen.getByLabelText(/Noon/i)).not.toBeChecked()
  })

  it('calls updatePlan on submit', async () => {
    const onSave = jest.fn()
    render(<PlanForm plan={existingPlan} onSave={onSave} onCancel={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /Save/i }))
    await waitFor(() => expect(mockUpdate).toHaveBeenCalled())
    expect(onSave).toHaveBeenCalledWith('plan-1')
  })

  it('calls onCancel when cancel is clicked', () => {
    const onCancel = jest.fn()
    render(<PlanForm plan={existingPlan} onSave={jest.fn()} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: /Cancel/i }))
    expect(onCancel).toHaveBeenCalled()
  })

  it('toggles iCal hide slot checkbox', () => {
    render(<PlanForm plan={existingPlan} onSave={jest.fn()} onCancel={jest.fn()} />)
    const noonCheckbox = screen.getByLabelText(/Noon/i)
    expect(noonCheckbox).not.toBeChecked()
    fireEvent.click(noonCheckbox)
    expect(noonCheckbox).toBeChecked()
  })
})
