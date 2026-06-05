import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { Button } from '@/components/ui/button'

describe('Button', () => {
  it('renders a button with its children', () => {
    render(<Button>Click me</Button>)
    expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument()
  })

  it('uses the rounded-xl (squared) shape by default, not a pill', () => {
    render(<Button>Save</Button>)
    const btn = screen.getByRole('button', { name: 'Save' })
    expect(btn).toHaveClass('rounded-xl')
    expect(btn).not.toHaveClass('rounded-full')
  })

  it('applies the default accent variant', () => {
    render(<Button>Save</Button>)
    expect(screen.getByRole('button')).toHaveClass('bg-accent')
  })

  it('applies the requested variant and size classes', () => {
    render(
      <Button variant="destructive" size="sm">
        Delete
      </Button>
    )
    const btn = screen.getByRole('button', { name: 'Delete' })
    expect(btn).toHaveClass('bg-danger')
    expect(btn).toHaveClass('h-8')
  })

  it('renders the small icon size as a squared rounded-lg control', () => {
    render(
      <Button size="iconSm" aria-label="remove">
        ×
      </Button>
    )
    const btn = screen.getByRole('button', { name: 'remove' })
    expect(btn).toHaveClass('h-6', 'w-6', 'rounded-lg')
  })

  it('lets className override conflicting variant utilities', () => {
    render(
      <Button variant="ghost" className="text-danger">
        x
      </Button>
    )
    const btn = screen.getByRole('button', { name: 'x' })
    expect(btn).toHaveClass('text-danger')
    expect(btn).not.toHaveClass('text-fg')
  })

  it('fires onClick', () => {
    const onClick = jest.fn()
    render(<Button onClick={onClick}>Go</Button>)
    fireEvent.click(screen.getByRole('button', { name: 'Go' }))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('renders the child element when asChild is set, inheriting button styles', () => {
    render(
      <Button asChild>
        <a href="/somewhere">Link button</a>
      </Button>
    )
    const link = screen.getByRole('link', { name: 'Link button' })
    expect(link).toHaveAttribute('href', '/somewhere')
    expect(link).toHaveClass('bg-accent', 'rounded-xl')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
