import React, { useState } from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Combobox } from '@/components/ui/combobox'

function Harness({
  suggestions,
  onSelect
}: {
  suggestions: string[]
  onSelect?: (value: string) => void
}) {
  const [value, setValue] = useState('')
  return (
    <Combobox
      value={value}
      onChange={setValue}
      onSelect={(v) => {
        setValue(v)
        onSelect?.(v)
      }}
      suggestions={suggestions}
      placeholder="Pick one"
      aria-label="picker"
    />
  )
}

const suggestions = ['Apple', 'Apricot', 'Banana']

describe('Combobox', () => {
  it('shows filtered suggestions when typing', () => {
    render(<Harness suggestions={suggestions} />)
    fireEvent.change(screen.getByLabelText('picker'), { target: { value: 'ap' } })
    expect(screen.getByText('Apple')).toBeInTheDocument()
    expect(screen.getByText('Apricot')).toBeInTheDocument()
    expect(screen.queryByText('Banana')).not.toBeInTheDocument()
  })

  it('hides the dropdown once the input exactly matches a suggestion', () => {
    render(<Harness suggestions={suggestions} />)
    fireEvent.change(screen.getByLabelText('picker'), { target: { value: 'Apple' } })
    expect(screen.queryByText('Apple')).not.toBeInTheDocument()
  })

  it('selects a suggestion on click and fills the input', () => {
    const onSelect = jest.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    fireEvent.change(screen.getByLabelText('picker'), { target: { value: 'ap' } })
    fireEvent.mouseDown(screen.getByText('Apricot'))
    expect(onSelect).toHaveBeenCalledWith('Apricot')
    expect((screen.getByLabelText('picker') as HTMLInputElement).value).toBe('Apricot')
  })

  it('snaps to an exact case-insensitive match on blur', async () => {
    const onSelect = jest.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    const input = screen.getByLabelText('picker')
    fireEvent.change(input, { target: { value: 'banana' } })
    fireEvent.blur(input)
    await waitFor(() => expect(onSelect).toHaveBeenCalledWith('Banana'))
  })

  it('navigates with arrow keys and selects with Enter', () => {
    const onSelect = jest.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    const input = screen.getByLabelText('picker')
    fireEvent.change(input, { target: { value: 'ap' } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSelect).toHaveBeenCalledWith('Apple')
  })
})
