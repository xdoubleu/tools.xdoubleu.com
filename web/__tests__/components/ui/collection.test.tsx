import { fireEvent, render, screen } from '@testing-library/react'
import { CollectionGrid } from '@/components/ui/collection-grid'
import { CollectionToolbar } from '@/components/ui/collection-toolbar'
import { ItemCard } from '@/components/ui/item-card'

describe('ItemCard', () => {
  it('links to the item with its title, meta and status', () => {
    render(
      <ItemCard
        href="/recipes/r1"
        title="Pasta"
        artwork={<span>art</span>}
        meta="Serves 2"
        status={<span>Draft</span>}
        actions={<button type="button">Delete</button>}
      />
    )
    const link = screen.getByRole('link', { name: /Pasta/ })
    expect(link).toHaveAttribute('href', '/recipes/r1')
    expect(link).toHaveTextContent('Serves 2')
    expect(link).toHaveTextContent('Draft')
    expect(link).toHaveTextContent('art')
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
  })

  it('omits the meta and status lines when absent', () => {
    render(<ItemCard href="/x" title="Plain" />)
    const link = screen.getByRole('link', { name: 'Plain' })
    expect(link.querySelectorAll('p')).toHaveLength(1)
    expect(link.querySelectorAll('div')).toHaveLength(2)
  })
})

describe('ItemCard without a link', () => {
  it('renders a static card with its actions', () => {
    render(<ItemCard title="Show" meta="Host" actions={<button type="button">Remove</button>} />)
    expect(document.querySelector('a')).toBeNull()
    expect(screen.getByText('Show')).toBeInTheDocument()
    expect(screen.getByText('Host')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument()
  })

  it('has no actions row without actions', () => {
    const { container } = render(<ItemCard title="Show" />)
    expect(container.querySelector('.flex-wrap')).toBeNull()
  })
})

describe('CollectionGrid', () => {
  it('lays children out one, two then three columns wide', () => {
    render(
      <CollectionGrid>
        <p>child</p>
      </CollectionGrid>
    )
    expect(screen.getByText('child').parentElement).toHaveClass(
      'grid-cols-1',
      'sm:grid-cols-2',
      'lg:grid-cols-3'
    )
  })
})

describe('CollectionToolbar', () => {
  it('searches, toggles facets and counts results', () => {
    const onQueryChange = vi.fn()
    const onToggle = vi.fn()
    render(
      <CollectionToolbar
        noun="recipes"
        query="pa"
        onQueryChange={onQueryChange}
        facets={[{ id: 'drafts', label: 'Drafts', active: true, onToggle }]}
        count={2}
      />
    )

    const search = screen.getByRole('searchbox', { name: 'Search recipes' })
    expect(search).toHaveValue('pa')
    expect(search).toHaveAttribute('placeholder', 'Search recipes')
    fireEvent.change(search, { target: { value: 'pas' } })
    expect(onQueryChange).toHaveBeenCalledWith('pas')

    const chip = screen.getByRole('button', { name: /Drafts/ })
    expect(chip).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(chip)
    expect(onToggle).toHaveBeenCalledTimes(1)

    expect(screen.getByRole('status')).toHaveTextContent(/^2 results$/)
  })

  it('renders extra controls between the facets and the count', () => {
    render(
      <CollectionToolbar noun="titles" query="" onQueryChange={vi.fn()} count={3}>
        <span>extra control</span>
      </CollectionToolbar>
    )
    expect(screen.getByText('extra control')).toBeInTheDocument()
  })

  it('uses the singular for one result and needs no facets', () => {
    render(<CollectionToolbar noun="books" query="" onQueryChange={vi.fn()} count={1} />)
    expect(screen.getByRole('status')).toHaveTextContent(/^1 result$/)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
