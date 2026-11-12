import { render, screen } from '@testing-library/react'
import { ChefHat } from 'lucide-react'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

describe('LoadingState', () => {
  it('says Loading… by default', () => {
    render(<LoadingState />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading…')
  })

  it('names what is loading', () => {
    render(<LoadingState label="recipe" />)
    expect(screen.getByText('Loading recipe…')).toHaveClass('text-sm', 'text-muted')
  })

  it('centres itself with page placement', () => {
    render(<LoadingState placement="page" />)
    expect(screen.getByRole('status')).toHaveClass('py-16', 'text-center')
  })
})

describe('ErrorState', () => {
  it('reports what failed to load', () => {
    render(<ErrorState what="books" />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load books.')
  })

  it('sits in flow by default and centres with page placement', () => {
    const { rerender } = render(<ErrorState what="books" />)
    expect(screen.getByRole('alert')).not.toHaveClass('py-16')
    rerender(<ErrorState what="books" placement="page" />)
    expect(screen.getByRole('alert')).toHaveClass('py-16', 'text-center')
  })
})

describe('EmptyState', () => {
  it('draws a muted icon above the message when given one', () => {
    const { container } = render(<EmptyState icon={ChefHat}>No recipes.</EmptyState>)
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })

  it('draws no icon by default', () => {
    const { container } = render(<EmptyState>No recipes.</EmptyState>)
    expect(container.querySelector('svg')).toBeNull()
  })

  it('renders the message and optional action', () => {
    render(<EmptyState action={<button>New</button>}>No recipes yet.</EmptyState>)
    expect(screen.getByText('No recipes yet.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'New' })).toBeInTheDocument()
  })

  it('renders without an action', () => {
    const { container } = render(<EmptyState>Nothing here.</EmptyState>)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(container.firstElementChild?.children).toHaveLength(1)
  })

  it('centres the action with page placement', () => {
    render(<EmptyState action={<button>New</button>}>Nothing here.</EmptyState>)
    expect(screen.getByRole('button', { name: 'New' }).parentElement).toHaveClass(
      'flex',
      'justify-center'
    )
  })

  it('centres a padded block by default', () => {
    const { container } = render(<EmptyState>Nothing here.</EmptyState>)
    expect(container.firstElementChild).toHaveClass('py-16', 'text-center')
  })

  it('sits flush and left-aligned inline', () => {
    const { container } = render(
      <EmptyState placement="inline" action={<button>New</button>}>
        Nothing here.
      </EmptyState>
    )
    expect(container.firstElementChild).not.toHaveClass('py-16', 'text-center')
    expect(container.firstElementChild).toHaveClass('space-y-2')
    expect(screen.getByRole('button', { name: 'New' }).parentElement).not.toHaveClass(
      'justify-center'
    )
  })
})
