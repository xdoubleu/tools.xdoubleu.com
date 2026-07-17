import { render, screen } from '@testing-library/react'
import CategoryBadge from '@/components/reading/CategoryBadge'

describe('CategoryBadge', () => {
  it('renders nothing for books (the default category)', () => {
    const { container } = render(<CategoryBadge category="book" />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing for empty/legacy categories', () => {
    const { container } = render(<CategoryBadge category="" />)
    expect(container).toBeEmptyDOMElement()
  })

  it.each([
    ['paper', 'Paper'],
    ['article', 'Article'],
    ['rss', 'RSS']
  ])('renders a %s badge', (category, label) => {
    render(<CategoryBadge category={category} />)
    expect(screen.getByText(label)).toBeInTheDocument()
  })
})
