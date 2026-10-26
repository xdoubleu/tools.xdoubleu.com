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

  it('centres a padded block by default', () => {
    const { container } = render(<EmptyState>Nothing here.</EmptyState>)
    expect(container.firstElementChild).toHaveClass('py-8', 'text-center')
  })

  it('sits flush and left-aligned when compact', () => {
    const { container } = render(
      <EmptyState compact action={<button>New</button>}>
        Nothing here.
      </EmptyState>
    )
    expect(container.firstElementChild).not.toHaveClass('py-8', 'text-center')
    expect(container.firstElementChild).toHaveClass('space-y-2')
    expect(screen.getByRole('button', { name: 'New' }).parentElement).not.toHaveClass(
      'justify-center'
    )
  })
})
