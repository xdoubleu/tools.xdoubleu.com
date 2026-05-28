import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@/hooks/useMealPlans', () => ({
  useCreatePlan: jest.fn(),
  useUpdatePlan: jest.fn()
}))
jest.mock('@/lib/gen/mealplans/v1/mealplans_pb', () => ({
  ...jest.requireActual('@/lib/gen/mealplans/v1/mealplans_pb'),
  CreatePlanRequest: jest.fn().mockImplementation((d) => d),
  UpdatePlanRequest: jest.fn().mockImplementation((d) => d)
}))

import { useCreatePlan, useUpdatePlan } from '@/hooks/useMealPlans'
import PlanForm from '@/components/recipes/PlanForm'
import { create } from '@bufbuild/protobuf'
import { PlanSchema } from '@/lib/gen/mealplans/v1/mealplans_pb'

const mockCreate = jest.fn()
const mockUpdate = jest.fn()

beforeEach(() => {
  jest.clearAllMocks()
  jest.mocked(useCreatePlan).mockReturnValue(mockCreate)
  jest.mocked(useUpdatePlan).mockReturnValue(mockUpdate)
  mockCreate.mockResolvedValue({ plan: { id: 'new-plan-id' } })
  mockUpdate.mockResolvedValue({})
})

describe('PlanForm (create mode)', () => {
  it('renders name input and iCal controls', () => {
    render(<PlanForm onSave={jest.fn()} onCancel={jest.fn()} />)
    expect(screen.getByLabelText(/Plan Name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Breakfast/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Noon/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Evening/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/Hide past events/i)).toBeInTheDocument()
  })

  it('calls createPlan with name on submit', async () => {
    const onSave = jest.fn()
    render(<PlanForm onSave={onSave} onCancel={jest.fn()} />)
    fireEvent.change(screen.getByLabelText(/Plan Name/i), { target: { value: 'My Plan' } })
    fireEvent.click(screen.getByRole('button', { name: /Save Plan/i }))
    await waitFor(() => expect(mockCreate).toHaveBeenCalled())
    expect(onSave).toHaveBeenCalledWith('new-plan-id')
  })

  it('calls onCancel when cancel is clicked', () => {
    const onCancel = jest.fn()
    render(<PlanForm onSave={jest.fn()} onCancel={onCancel} />)
    fireEvent.click(screen.getByRole('button', { name: /Cancel/i }))
    expect(onCancel).toHaveBeenCalled()
  })

  it('toggles iCal hide slot checkbox', () => {
    render(<PlanForm onSave={jest.fn()} onCancel={jest.fn()} />)
    const breakfastCheckbox = screen.getByLabelText(/Breakfast/i)
    expect(breakfastCheckbox).not.toBeChecked()
    fireEvent.click(breakfastCheckbox)
    expect(breakfastCheckbox).toBeChecked()
  })
})

describe('PlanForm (edit mode)', () => {
  const existingPlan = create(PlanSchema, {
    id: 'plan-1',
    name: 'Existing Plan',
    canEdit: true,
    icalHideSlots: ['breakfast'],
    icalHidePast: true
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
    fireEvent.click(screen.getByRole('button', { name: /Save Plan/i }))
    await waitFor(() => expect(mockUpdate).toHaveBeenCalled())
    expect(onSave).toHaveBeenCalledWith('plan-1')
  })
})
