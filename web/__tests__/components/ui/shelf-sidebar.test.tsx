import React from 'react'
import { render, screen, fireEvent, within } from '@testing-library/react'
import { ShelfSidebar } from '@/components/ui/shelf-sidebar'

const shelves = [
  { id: 'all', label: 'All', count: 3 },
  { id: 'done', label: 'Done', count: 1 }
]

function setup(props: Partial<React.ComponentProps<typeof ShelfSidebar>> = {}) {
  const onSelectShelf = mk()
  const onSelectTag = mk()
  render(
    <ShelfSidebar
      shelves={shelves}
      selectedShelfId="all"
      selectedTag={null}
      onSelectShelf={onSelectShelf}
      onSelectTag={onSelectTag}
      {...props}
    />
  )
  return { onSelectShelf, onSelectTag }
}
const mk = () => vi.fn()

describe('ShelfSidebar', () => {
  it('marks only the selected shelf as current and selects on click', () => {
    const { onSelectShelf } = setup()
    const nav = screen.getByRole('navigation', { name: 'Library navigation' })
    expect(within(nav).getByText('All').closest('button')).toHaveAttribute('aria-current', 'true')
    expect(within(nav).getByText('Done').closest('button')).not.toHaveAttribute('aria-current')
    expect(within(nav).getByText('All').closest('button')).toHaveClass('text-accent', 'rounded-xl')
    expect(within(nav).getByText('Done').closest('button')).toHaveClass('text-subtle')
    fireEvent.click(within(nav).getByText('Done'))
    expect(onSelectShelf).toHaveBeenCalledWith('done')
  })

  it('renders no tag section without tags', () => {
    setup()
    expect(screen.queryByText('Tags')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Tags')).not.toBeInTheDocument()
  })

  it('renders tags in both layouts, marks the active one and selects on click', () => {
    const { onSelectTag } = setup({
      selectedShelfId: null,
      selectedTag: 'fantasy',
      allTags: [
        { name: 'fantasy', count: 2 },
        { name: 'scifi', count: 1 }
      ]
    })
    const nav = screen.getByRole('navigation', { name: 'Library navigation' })
    expect(within(nav).getByText('fantasy').closest('button')).toHaveAttribute(
      'aria-current',
      'true'
    )
    expect(within(nav).getByText('scifi').closest('button')).not.toHaveAttribute('aria-current')
    const pills = within(screen.getByLabelText('Tags'))
    expect(pills.getByText('fantasy').closest('button')).toHaveAttribute('aria-pressed', 'true')
    expect(pills.getByText('scifi').closest('button')).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(screen.getAllByText('scifi')[1])
    expect(onSelectTag).toHaveBeenCalledWith('scifi')
    fireEvent.click(within(nav).getByText('scifi'))
    expect(onSelectTag).toHaveBeenCalledTimes(2)
  })

  it('selects no mobile tab when nothing is selected', () => {
    setup({ selectedShelfId: null })
    for (const tab of screen.getAllByRole('tab')) {
      expect(tab).toHaveAttribute('aria-selected', 'false')
    }
  })

  it('links series with the given href builder', () => {
    setup({
      series: [{ name: 'Dune', read: 1, total: 6, href: '/s/Dune' }]
    })
    for (const a of screen.getAllByRole('link', { name: /Dune/ })) {
      expect(a).toHaveAttribute('href', '/s/Dune')
    }
  })
})
