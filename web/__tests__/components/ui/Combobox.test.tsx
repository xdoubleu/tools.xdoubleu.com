import React, { useState } from 'react'
import { render, screen, fireEvent, waitFor, within, createEvent } from '@testing-library/react'
import { Combobox } from '@/components/ui/combobox'

function Harness({
  suggestions,
  onSelect,
  filterSuggestions
}: {
  suggestions: string[]
  onSelect?: (value: string) => void
  filterSuggestions?: boolean
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
      filterSuggestions={filterSuggestions}
      placeholder="Pick one"
      aria-label="picker"
    />
  )
}

const suggestions = ['Apple', 'Apricot', 'Banana']

describe('Combobox', () => {
  it('shows suggestions as given when filterSuggestions is false', () => {
    render(<Harness suggestions={suggestions} filterSuggestions={false} />)
    fireEvent.change(screen.getByLabelText('picker'), { target: { value: 'ap' } })
    expect(screen.getByText('Banana')).toBeInTheDocument()
  })

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
    const onSelect = vi.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    fireEvent.change(screen.getByLabelText('picker'), { target: { value: 'ap' } })
    fireEvent.mouseDown(screen.getByText('Apricot'))
    expect(onSelect).toHaveBeenCalledWith('Apricot')
    expect((screen.getByLabelText('picker') as HTMLInputElement).value).toBe('Apricot')
  })

  it('snaps to an exact case-insensitive match on blur', async () => {
    const onSelect = vi.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    const input = screen.getByLabelText('picker')
    fireEvent.change(input, { target: { value: 'banana' } })
    fireEvent.blur(input)
    await waitFor(() => expect(onSelect).toHaveBeenCalledWith('Banana'))
  })

  it('navigates with arrow keys and selects with Enter', () => {
    const onSelect = vi.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    const input = screen.getByLabelText('picker')
    fireEvent.change(input, { target: { value: 'ap' } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSelect).toHaveBeenCalledWith('Apple')
  })

  it('exposes combobox ARIA wired to its listbox', () => {
    render(<Harness suggestions={suggestions} />)
    const input = screen.getByRole('combobox', { name: 'picker' })
    expect(input).toHaveAttribute('aria-autocomplete', 'list')
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(input).not.toHaveAttribute('aria-controls')

    fireEvent.change(input, { target: { value: 'ap' } })
    expect(input).toHaveAttribute('aria-expanded', 'true')
    const listbox = screen.getByRole('listbox')
    expect(input).toHaveAttribute('aria-controls', listbox.id)
    const options = within(listbox).getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual(['Apple', 'Apricot'])
    expect(options.every((o) => o.getAttribute('aria-selected') === 'false')).toBe(true)
    expect(input).not.toHaveAttribute('aria-activedescendant')
  })

  it('points aria-activedescendant at the highlighted option', () => {
    render(<Harness suggestions={suggestions} />)
    const input = screen.getByRole('combobox')
    fireEvent.change(input, { target: { value: 'ap' } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    const highlighted = screen.getByRole('option', { name: 'Apricot' })
    expect(highlighted).toHaveAttribute('aria-selected', 'true')
    expect(input).toHaveAttribute('aria-activedescendant', highlighted.id)
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(input).not.toHaveAttribute('aria-activedescendant')
  })

  it('lets Tab move focus without committing when nothing is highlighted', () => {
    const onSelect = vi.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    const input = screen.getByRole('combobox')
    fireEvent.change(input, { target: { value: 'ap' } })
    const tab = createEvent.keyDown(input, { key: 'Tab' })
    fireEvent(input, tab)
    expect(tab.defaultPrevented).toBe(false)
    expect(onSelect).not.toHaveBeenCalled()
    expect(input).toHaveValue('ap')
  })

  it('commits the highlighted option on Tab without blocking the focus move', () => {
    const onSelect = vi.fn()
    render(<Harness suggestions={suggestions} onSelect={onSelect} />)
    const input = screen.getByRole('combobox')
    fireEvent.change(input, { target: { value: 'ap' } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    const tab = createEvent.keyDown(input, { key: 'Tab' })
    fireEvent(input, tab)
    expect(tab.defaultPrevented).toBe(false)
    expect(onSelect).toHaveBeenCalledWith('Apple')
    expect(input).toHaveAttribute('aria-expanded', 'false')
  })
})
