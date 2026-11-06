import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'

const mockModels = vi.fn()
vi.mock('@/hooks/useFactory', () => ({
  useFactoryModels: () => mockModels()
}))

import { FactoryModelPicker } from '@/components/factory/FactoryModelPicker'

beforeEach(() => {
  vi.clearAllMocks()
  mockModels.mockReturnValue({ data: ['vendor/alpha'] })
})

describe('FactoryModelPicker', () => {
  it('renders a labeled field with the model value', () => {
    const onChange = vi.fn()
    render(
      <FactoryModelPicker
        id="model"
        label="Default model"
        hint="A hint"
        value="vendor/old"
        onChange={onChange}
      />
    )
    expect(screen.getByLabelText('Default model')).toHaveValue('vendor/old')
  })

  it('offers the catalog models as suggestions once something is typed', () => {
    render(
      <FactoryModelPicker
        id="model"
        label="Default model"
        hint="A hint"
        value=""
        onChange={vi.fn()}
      />
    )
    const input = screen.getByLabelText('Default model')
    // An exact in-catalog match snaps in as a suggestion isn't shown, so type a prefix.
    fireEvent.change(input, { target: { value: 'vendor/al' } })
    expect(screen.getByText('vendor/alpha')).toBeInTheDocument()
  })

  it('keeps a typed id that is not in the catalog', () => {
    const onChange = vi.fn()
    render(
      <FactoryModelPicker
        id="model"
        label="Default model"
        hint="A hint"
        value=""
        onChange={onChange}
      />
    )
    const input = screen.getByLabelText('Default model')
    fireEvent.change(input, { target: { value: 'custom/v1' } })
    expect(onChange).toHaveBeenCalledWith('custom/v1')
  })

  it('marks the field required and turns off mobile text insertion when asked', () => {
    render(
      <FactoryModelPicker
        id="model"
        label="Default model"
        hint="A hint"
        value=""
        onChange={vi.fn()}
        required
      />
    )
    const input = screen.getByLabelText('Default model') as HTMLInputElement
    expect(input.required).toBe(true)
    expect(input).toHaveAttribute('autocapitalize', 'none')
    expect(input).toHaveAttribute('autocorrect', 'off')
  })

  it('leaves the field optional by default', () => {
    render(
      <FactoryModelPicker
        id="model"
        label="Default model"
        hint="A hint"
        value=""
        onChange={vi.fn()}
      />
    )
    expect((screen.getByLabelText('Default model') as HTMLInputElement).required).toBe(false)
  })

  it('offers no suggestions before the catalog loads', () => {
    mockModels.mockReturnValue({ data: undefined })
    render(
      <FactoryModelPicker
        id="model"
        label="Default model"
        hint="A hint"
        value=""
        onChange={vi.fn()}
      />
    )
    // With no catalog the dropdown must not open at all.
    fireEvent.keyDown(screen.getByLabelText('Default model'), { key: 'ArrowDown' })
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })
})
