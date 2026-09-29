import { render, screen, within } from '@testing-library/react'
import FeedItemCategories from '@/components/feeds/FeedItemCategories'

describe('FeedItemCategories', () => {
  it('renders one chip per category in a labelled list', () => {
    render(<FeedItemCategories categories={['Product announcements', 'Claude Code']} />)
    const list = screen.getByRole('list', { name: 'Categories' })
    const chips = within(list).getAllByRole('listitem')
    expect(chips.map((c) => c.textContent)).toEqual(['Product announcements', 'Claude Code'])
  })

  it('renders nothing without categories', () => {
    const { container } = render(<FeedItemCategories categories={[]} />)
    expect(container).toBeEmptyDOMElement()
  })
})
