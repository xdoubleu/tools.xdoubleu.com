import { render, screen } from '@testing-library/react'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

describe('LoadingState', () => {
  it('says Loading… by default', () => {
    render(<LoadingState />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading…')
  })

  it('names what is loading', () => {
    render(<LoadingState label="recipe" />)
    expect(screen.getByText('Loading recipe…')).toHaveClass('text-muted')
  })
})

describe('ErrorState', () => {
  it('reports what failed to load', () => {
    render(<ErrorState what="books" />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load books.')
  })
})

describe('EmptyState', () => {
  it('renders the message and optional action', () => {
    render(<EmptyState action={<button>New</button>}>No recipes yet.</EmptyState>)
    expect(screen.getByText('No recipes yet.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'New' })).toBeInTheDocument()
  })

  it('renders without an action', () => {
    render(<EmptyState>Nothing here.</EmptyState>)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
