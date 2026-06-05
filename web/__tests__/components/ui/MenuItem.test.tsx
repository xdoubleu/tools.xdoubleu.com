import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { MenuItem } from '@/components/ui/menu-item'

describe('MenuItem', () => {
  it('renders a button defaulting to type="button"', () => {
    render(<MenuItem>Option</MenuItem>)
    const item = screen.getByRole('button', { name: 'Option' })
    expect(item).toHaveAttribute('type', 'button')
  })

  it('uses the shared menu-item styling', () => {
    render(<MenuItem>Option</MenuItem>)
    const item = screen.getByRole('button', { name: 'Option' })
    expect(item).toHaveClass('w-full', 'rounded-lg', 'text-left', 'hover:bg-hover')
  })

  it('fires onClick', () => {
    const onClick = jest.fn()
    render(<MenuItem onClick={onClick}>Option</MenuItem>)
    fireEvent.click(screen.getByRole('button', { name: 'Option' }))
    expect(onClick).toHaveBeenCalledTimes(1)
  })
})
