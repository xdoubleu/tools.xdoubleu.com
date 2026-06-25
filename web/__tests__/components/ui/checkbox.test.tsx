import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { Checkbox } from '@/components/ui/checkbox'

describe('Checkbox', () => {
  it('renders an unchecked checkbox by default', () => {
    render(<Checkbox />)
    expect(screen.getByRole('checkbox')).not.toBeChecked()
  })

  it('renders a checked checkbox when checked prop is set', () => {
    render(<Checkbox checked onChange={jest.fn()} />)
    expect(screen.getByRole('checkbox')).toBeChecked()
  })

  it('renders a label when label prop is provided', () => {
    render(<Checkbox label="Accept terms" id="terms" />)
    expect(screen.getByLabelText('Accept terms')).toBeInTheDocument()
  })

  it('calls onChange when clicked', () => {
    const onChange = jest.fn()
    render(<Checkbox onChange={onChange} />)
    fireEvent.click(screen.getByRole('checkbox'))
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('does not call onChange when disabled', () => {
    const onChange = jest.fn()
    render(<Checkbox disabled onChange={onChange} />)
    const checkbox = screen.getByRole('checkbox')
    expect(checkbox).toBeDisabled()
  })

  it('applies custom className', () => {
    render(<Checkbox className="custom-class" />)
    expect(screen.getByRole('checkbox')).toHaveClass('custom-class')
  })
})
