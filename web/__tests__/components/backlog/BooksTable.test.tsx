import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'
import BooksTable from '@/components/backlog/BooksTable'

jest.mock('next/image', () => {
  return function MockImage({ src, alt }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} />
  }
})

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/backlog/BookRatingStars', () => {
  return function MockRatingStars({ userBook }: { userBook: { rating: number } }) {
    return <div data-testid="rating-stars" data-rating={userBook.rating} />
  }
})

jest.mock('@/components/backlog/BookFavouriteButton', () => {
  return function MockFavButton() {
    return <div data-testid="favourite-button" />
  }
})

jest.mock('@/components/backlog/BookShelfTagCell', () => {
  return function MockShelfTagCell() {
    return <div data-testid="shelf-tag-cell" />
  }
})

jest.mock('@/components/backlog/BookOwnershipToggles', () => {
  return function MockOwnershipToggles() {
    return <div data-testid="ownership-toggles" />
  }
})

function makeBook(id: string, title: string, author = 'Author', overrides = {}) {
  return create(UserBookSchema, {
    id,
    status: 'to-read',
    tags: [],
    formats: [],
    addedAt: '2024-01-01T00:00:00Z',
    book: create(BookSchema, { title, authors: [author], pageCount: 300 }),
    ...overrides
  })
}

describe('BooksTable', () => {
  it('renders a table with all column headers', () => {
    render(<BooksTable books={[]} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Title' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Author' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Pages' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'ISBN' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rating' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Owned' })).toBeInTheDocument()
  })

  it('renders isbn13 value in the ISBN column', () => {
    const book = create(UserBookSchema, {
      id: '1',
      status: 'to-read',
      tags: [],
      formats: [],
      addedAt: '2024-01-01T00:00:00Z',
      book: create(BookSchema, {
        title: 'Dune',
        authors: ['Frank Herbert'],
        pageCount: 300,
        isbn13: '9780441013593'
      })
    })
    render(<BooksTable books={[book]} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('9780441013593')).toBeInTheDocument()
  })

  it('renders ownership toggles for each book row', () => {
    const books = [makeBook('1', 'Dune'), makeBook('2', 'Hyperion')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    expect(screen.getAllByTestId('ownership-toggles')).toHaveLength(2)
  })

  it('renders books in rows', () => {
    const books = [makeBook('1', 'Dune', 'Frank Herbert'), makeBook('2', 'Hyperion')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Hyperion')).toBeInTheDocument()
    expect(screen.getByText('Frank Herbert')).toBeInTheDocument()
  })

  it('shows empty message when no books', () => {
    render(<BooksTable books={[]} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('No books match the current filters.')).toBeInTheDocument()
  })

  it('title links to the book detail page', () => {
    const books = [makeBook('abc', 'Dune')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    const link = screen.getByRole('link', { name: 'Dune' })
    expect(link).toHaveAttribute('href', '/backlog/books/abc')
  })

  it('author links to the author page', () => {
    const books = [makeBook('1', 'Dune', 'Frank Herbert')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    const link = screen.getByRole('link', { name: 'Frank Herbert' })
    expect(link).toHaveAttribute('href', '/backlog/books/author/Frank%20Herbert')
  })

  it('sorts by title asc when Title header is clicked once', () => {
    const books = [makeBook('z', 'Zebra'), makeBook('a', 'Aardvark')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Title' }))
    const cells = screen.getAllByRole('cell')
    const titles = cells.map((c) => c.textContent).filter((t) => t === 'Aardvark' || t === 'Zebra')
    expect(titles[0]).toBe('Aardvark')
  })

  it('sorts by title desc when Title header is clicked twice', () => {
    const books = [makeBook('a', 'Aardvark'), makeBook('z', 'Zebra')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Title' }))
    fireEvent.click(screen.getByRole('button', { name: 'Title' }))
    const cells = screen.getAllByRole('cell')
    const titles = cells.map((c) => c.textContent).filter((t) => t === 'Aardvark' || t === 'Zebra')
    expect(titles[0]).toBe('Zebra')
  })

  it('shows pagination when books exceed page size', () => {
    const books = Array.from({ length: 25 }, (_, i) => makeBook(`b${i}`, `Book ${i}`))
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled()
  })

  it('hides pagination when books fit on one page', () => {
    const books = [makeBook('1', 'One')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument()
  })

  it('navigates to next page and back with Prev', () => {
    const books = Array.from({ length: 25 }, (_, i) => makeBook(`b${i}`, `Book ${i}`))
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('2 / 2')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Prev' }))
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
  })

  it('sorts by author asc when Author header is clicked', () => {
    const books = [makeBook('1', 'Book A', 'Zoe'), makeBook('2', 'Book B', 'Alice')]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Author' }))
    const cells = screen.getAllByRole('cell')
    const authors = cells.map((c) => c.textContent).filter((t) => t === 'Alice' || t === 'Zoe')
    expect(authors[0]).toBe('Alice')
  })

  it('sorts by pages asc when Pages header is clicked', () => {
    const books = [
      makeBook('1', 'Big Book', 'A', {
        book: create(BookSchema, { title: 'Big Book', authors: ['A'], pageCount: 500 })
      }),
      makeBook('2', 'Short Book', 'B', {
        book: create(BookSchema, { title: 'Short Book', authors: ['B'], pageCount: 100 })
      })
    ]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Pages' }))
    const cells = screen.getAllByRole('cell')
    const pages = cells.map((c) => c.textContent).filter((t) => t === '500' || t === '100')
    expect(pages[0]).toBe('100')
  })

  it('sorts by rating when Rating header is clicked', () => {
    const books = [
      makeBook('1', 'High', 'A', { rating: 5 }),
      makeBook('2', 'Low', 'B', { rating: 1 })
    ]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Rating' }))
    // After asc sort: rating 1 first, rating 5 second
    const ratingCells = screen.getAllByTestId('rating-stars')
    expect(ratingCells[0]).toHaveAttribute('data-rating', '1')
  })

  it('sorts by favourite when Fav header is clicked', () => {
    const books = [
      makeBook('1', 'Not Fav', 'A', { tags: [] }),
      makeBook('2', 'Fav', 'B', { tags: ['favourite'] })
    ]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Fav' }))
    // asc: non-fav (0) before fav (1)
    const cells = screen.getAllByRole('cell')
    const titles = cells.map((c) => c.textContent).filter((t) => t === 'Not Fav' || t === 'Fav')
    expect(titles[0]).toBe('Not Fav')
  })

  it('sorts by shelf when Shelf & tags header is clicked', () => {
    const books = [
      makeBook('1', 'Z Book', 'A', { status: 'to-read' }),
      makeBook('2', 'A Book', 'B', { status: 'currently-reading' })
    ]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Shelf & tags' }))
    // 'currently-reading' < 'to-read' alphabetically
    const cells = screen.getAllByRole('cell')
    const titles = cells.map((c) => c.textContent).filter((t) => t === 'Z Book' || t === 'A Book')
    expect(titles[0]).toBe('A Book')
  })

  it('sorts by date added when Date added header is clicked', () => {
    const books = [
      makeBook('1', 'Newer', 'A', { addedAt: '2024-06-01T00:00:00Z' }),
      makeBook('2', 'Older', 'B', { addedAt: '2023-01-01T00:00:00Z' })
    ]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Date added' }))
    const cells = screen.getAllByRole('cell')
    const titles = cells.map((c) => c.textContent).filter((t) => t === 'Newer' || t === 'Older')
    expect(titles[0]).toBe('Older')
  })

  it('sorts by date read when Date read header is clicked', () => {
    const books = [
      makeBook('1', 'Early Read', 'A', { finishedAt: ['2022-01-01T00:00:00Z'] }),
      makeBook('2', 'Late Read', 'B', { finishedAt: ['2025-01-01T00:00:00Z'] })
    ]
    render(<BooksTable books={books} knownShelves={[]} knownTags={[]} />)
    fireEvent.click(screen.getByRole('button', { name: 'Date read' }))
    const cells = screen.getAllByRole('cell')
    const titles = cells
      .map((c) => c.textContent)
      .filter((t) => t === 'Early Read' || t === 'Late Read')
    expect(titles[0]).toBe('Early Read')
  })

  it('cycles sort direction through asc -> desc -> none on triple click', () => {
    render(<BooksTable books={[makeBook('1', 'A')]} knownShelves={[]} knownTags={[]} />)
    const btn = screen.getByRole('button', { name: 'Author' })
    fireEvent.click(btn) // asc
    fireEvent.click(btn) // desc
    fireEvent.click(btn) // null — back to default
    // No sort indicator visible (no ^ or v text beside Author)
    expect(btn.textContent).toBe('Author')
  })
})
