import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import TogglePill from '@/components/reading/TogglePill'

describe('TogglePill', () => {
  it('renders the label', () => {
    render(<TogglePill label="Physical" active={false} onClick={jest.fn()} />)
    expect(screen.getByRole('button', { name: 'Physical' })).toBeInTheDocument()
  })

  it('reflects active state via aria-pressed', () => {
    render(<TogglePill label="Physical" active={true} onClick={jest.fn()} />)
    expect(screen.getByRole('button', { name: 'Physical' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('reflects inactive state via aria-pressed', () => {
    render(<TogglePill label="Physical" active={false} onClick={jest.fn()} />)
    expect(screen.getByRole('button', { name: 'Physical' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('fires onClick when clicked', () => {
    const onClick = jest.fn()
    render(<TogglePill label="Physical" active={false} onClick={onClick} />)
    fireEvent.click(screen.getByRole('button', { name: 'Physical' }))
    expect(onClick).toHaveBeenCalled()
  })
})
