import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { Input } from '@/components/ui/input'

describe('Input', () => {
  it('renders with rounded-xl shape and input tokens', () => {
    render(<Input aria-label="name" />)
    const input = screen.getByLabelText('name')
    expect(input).toHaveClass('rounded-xl', 'border-input-border', 'bg-input')
  })

  it('lets className override the default width', () => {
    render(<Input aria-label="amount" className="w-16" />)
    const input = screen.getByLabelText('amount')
    expect(input).toHaveClass('w-16')
    expect(input).not.toHaveClass('w-full')
  })

  it('forwards value and change events', () => {
    const onChange = jest.fn()
    render(<Input aria-label="q" value="" onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('q'), { target: { value: 'hi' } })
    expect(onChange).toHaveBeenCalled()
  })
})
