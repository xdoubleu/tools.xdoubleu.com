import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { TogglePill } from '@/components/ui/toggle-pill'

describe('TogglePill', () => {
  it('renders the label', () => {
    render(<TogglePill label="Physical" active={false} onClick={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Physical' })).toBeInTheDocument()
  })

  it('reflects active state via aria-pressed', () => {
    render(<TogglePill label="Physical" active={true} onClick={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Physical' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('reflects inactive state via aria-pressed', () => {
    render(<TogglePill label="Physical" active={false} onClick={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Physical' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it("keeps Button's squared corners", () => {
    render(<TogglePill label="Physical" active={false} onClick={vi.fn()} />)
    const pill = screen.getByRole('button', { name: 'Physical' })
    expect(pill).toHaveClass('rounded-xl')
    expect(pill).not.toHaveClass('rounded-full')
  })

  it('shares the control height', () => {
    render(<TogglePill label="Physical" active={false} onClick={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Physical' })).toHaveClass('h-11', 'sm:h-9')
  })

  it('fires onClick when clicked', () => {
    const onClick = vi.fn()
    render(<TogglePill label="Physical" active={false} onClick={onClick} />)
    fireEvent.click(screen.getByRole('button', { name: 'Physical' }))
    expect(onClick).toHaveBeenCalled()
  })

  it('marks the on state with a check and an accent tint', () => {
    render(<TogglePill label="Physical" active onClick={vi.fn()} />)
    const pill = screen.getByRole('button', { name: /Physical/ })
    expect(pill).toHaveTextContent('✓')
    expect(pill).toHaveClass('border-accent', 'text-accent')
  })

  it('stays an outline with no check when off', () => {
    render(<TogglePill label="Physical" active={false} onClick={vi.fn()} />)
    const pill = screen.getByRole('button', { name: 'Physical' })
    expect(pill).not.toHaveTextContent('✓')
    expect(pill).toHaveClass('border-border')
  })
})
