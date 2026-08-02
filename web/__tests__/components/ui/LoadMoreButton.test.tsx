import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'

describe('LoadMoreButton', () => {
  it('calls onClick when pressed', () => {
    const onClick = jest.fn()
    render(<LoadMoreButton onClick={onClick} loading={false} />)
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('shows a loading state and disables the button', () => {
    render(<LoadMoreButton onClick={jest.fn()} loading={true} />)
    const button = screen.getByRole('button', { name: 'Loading…' })
    expect(button).toBeDisabled()
  })
})
