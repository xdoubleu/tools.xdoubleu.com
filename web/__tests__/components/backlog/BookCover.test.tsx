import { render, screen, fireEvent } from '@testing-library/react'
import BookCover from '@/components/backlog/BookCover'

describe('BookCover', () => {
  it('renders the cover image when coverUrl is provided', () => {
    render(<BookCover coverUrl="https://example.com/cover.jpg" title="The Odyssey" />)
    const img = screen.getByRole('img')
    expect(img).toBeInTheDocument()
    expect(img).toHaveAttribute('alt', 'The Odyssey')
  })

  it('renders the placeholder when coverUrl is empty', () => {
    render(<BookCover coverUrl="" title="The Odyssey" />)
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    // Initials from "The Odyssey" → "TO"
    expect(screen.getByText('TO')).toBeInTheDocument()
  })

  it('renders the placeholder on image error', () => {
    render(<BookCover coverUrl="https://example.com/cover.jpg" title="My Book" />)
    const img = screen.getByRole('img')
    fireEvent.error(img)
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(screen.getByText('MB')).toBeInTheDocument()
  })

  it('derives initials from single-word title', () => {
    render(<BookCover coverUrl="" title="Dune" />)
    expect(screen.getByText('D')).toBeInTheDocument()
  })

  it('derives initials from multi-word title (only first two words)', () => {
    render(<BookCover coverUrl="" title="The Lord of the Rings" />)
    expect(screen.getByText('TL')).toBeInTheDocument()
  })

  it('renders the sm size box with correct dimensions', () => {
    const { container } = render(<BookCover coverUrl="" title="Book" size="sm" />)
    const box = container.firstElementChild
    expect(box).toHaveStyle({ width: '40px', height: '60px' })
  })

  it('renders the md size box with correct dimensions', () => {
    const { container } = render(<BookCover coverUrl="" title="Book" size="md" />)
    const box = container.firstElementChild
    expect(box).toHaveStyle({ width: '48px', height: '72px' })
  })

  it('always reserves space so layout is not affected by missing cover', () => {
    const { container } = render(<BookCover coverUrl="" title="Missing" size="sm" />)
    const box = container.firstElementChild
    // Box must have explicit size regardless of cover presence.
    expect(box).toHaveStyle({ width: '40px' })
  })
})
