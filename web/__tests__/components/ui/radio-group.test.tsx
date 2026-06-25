import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

function TestGroup({
  value = 'a',
  onChange = jest.fn()
}: {
  value?: string
  onChange?: (v: string) => void
}) {
  return (
    <RadioGroup name="test-group" value={value} onChange={onChange}>
      <RadioGroupItem value="a" label="Option A" />
      <RadioGroupItem value="b" label="Option B" />
    </RadioGroup>
  )
}

describe('RadioGroup', () => {
  it('renders all items', () => {
    render(<TestGroup />)
    expect(screen.getByLabelText('Option A')).toBeInTheDocument()
    expect(screen.getByLabelText('Option B')).toBeInTheDocument()
  })

  it('checks the item matching the current value', () => {
    render(<TestGroup value="b" />)
    expect(screen.getByLabelText('Option A')).not.toBeChecked()
    expect(screen.getByLabelText('Option B')).toBeChecked()
  })

  it('calls onChange with the new value when an item is clicked', () => {
    const onChange = jest.fn()
    render(<TestGroup value="a" onChange={onChange} />)
    fireEvent.click(screen.getByLabelText('Option B'))
    expect(onChange).toHaveBeenCalledWith('b')
  })

  it('has radiogroup role', () => {
    render(<TestGroup />)
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
  })

  it('throws when RadioGroupItem is used outside RadioGroup', () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<RadioGroupItem value="x" label="X" />)).toThrow(
      'RadioGroupItem must be inside RadioGroup'
    )
    spy.mockRestore()
  })
})
