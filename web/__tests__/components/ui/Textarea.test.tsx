import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { Textarea } from '@/components/ui/textarea'

describe('Textarea', () => {
  it('renders with rounded-xl shape and input tokens', () => {
    render(<Textarea aria-label="notes" />)
    const area = screen.getByLabelText('notes')
    expect(area.tagName).toBe('TEXTAREA')
    expect(area).toHaveClass('rounded-xl', 'border-input-border', 'bg-input')
  })

  it('uses 16px font on mobile to avoid iOS focus zoom', () => {
    render(<Textarea aria-label="notes" />)
    expect(screen.getByLabelText('notes')).toHaveClass('text-base', 'md:text-sm')
  })

  it('merges additional className', () => {
    render(<Textarea aria-label="notes" className="resize-none" />)
    expect(screen.getByLabelText('notes')).toHaveClass('resize-none')
  })

  it('forwards change events', () => {
    const onChange = jest.fn()
    render(<Textarea aria-label="notes" value="" onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('notes'), { target: { value: 'x' } })
    expect(onChange).toHaveBeenCalled()
  })
})
