import { render, screen } from '@testing-library/react'
import { Toolbar } from '@/components/ui/toolbar'

describe('Toolbar', () => {
  it('lays out search, filters and a trailing slot in one wrapping row', () => {
    render(
      <Toolbar aria-label="Filter books" search={<input aria-label="Search" />} trailing="12 books">
        <button>Unread</button>
      </Toolbar>
    )
    const toolbar = screen.getByRole('toolbar', { name: 'Filter books' })
    expect(toolbar).toHaveClass('flex-wrap')
    expect(screen.getByRole('textbox', { name: 'Search' }).parentElement).toHaveClass('basis-full')
    expect(screen.getByText('12 books')).toHaveClass('ml-auto')
    expect(screen.getByRole('button', { name: 'Unread' })).toBeInTheDocument()
  })

  it('omits empty slots', () => {
    render(<Toolbar>filters</Toolbar>)
    expect(screen.getByRole('toolbar').children).toHaveLength(0)
    expect(screen.getByRole('toolbar')).toHaveTextContent('filters')
  })
})
