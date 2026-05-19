import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import SettingsLabelRow from '@/components/todos/SettingsLabelRow'

describe('SettingsLabelRow', () => {
  const mockOnColorChange = jest.fn()
  const mockOnRemove = jest.fn()

  beforeEach(() => {
    mockOnColorChange.mockClear()
    mockOnRemove.mockClear()
  })

  it('renders the label value', () => {
    render(
      <SettingsLabelRow
        value="urgent"
        color="#ff0000"
        onColorChange={mockOnColorChange}
        onRemove={mockOnRemove}
      />
    )
    expect(screen.getByText('urgent')).toBeInTheDocument()
  })

  it('renders the color input with correct value', () => {
    render(
      <SettingsLabelRow
        value="urgent"
        color="#ff0000"
        onColorChange={mockOnColorChange}
        onRemove={mockOnRemove}
      />
    )
    const colorInput = screen.getByDisplayValue('#ff0000') as HTMLInputElement
    expect(colorInput).toHaveAttribute('type', 'color')
  })

  it('calls onColorChange when color is changed', () => {
    render(
      <SettingsLabelRow
        value="urgent"
        color="#ff0000"
        onColorChange={mockOnColorChange}
        onRemove={mockOnRemove}
      />
    )
    const colorInput = screen.getByDisplayValue('#ff0000') as HTMLInputElement
    fireEvent.change(colorInput, { target: { value: '#00ff00' } })
    expect(mockOnColorChange).toHaveBeenCalledWith('urgent', '#00ff00')
  })

  it('calls onRemove when Remove button is clicked', () => {
    render(
      <SettingsLabelRow
        value="urgent"
        color="#ff0000"
        onColorChange={mockOnColorChange}
        onRemove={mockOnRemove}
      />
    )
    const removeBtn = screen.getByRole('button', { name: 'Remove' })
    fireEvent.click(removeBtn)
    expect(mockOnRemove).toHaveBeenCalledWith('urgent')
  })
})
