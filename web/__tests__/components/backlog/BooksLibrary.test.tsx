import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import BooksLibrary from '@/components/backlog/BooksLibrary'
import {
  UserBookSchema,
  BookSchema,
  BookShelfSchema,
  LibraryResponseSchema
} from '@/lib/gen/backlog/v1/books_pb'

jest.mock('next/image', () => {
  return function MockImage({ src, alt, ...props }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} {...props} />
  }
})

jest.mock('@/components/backlog/BookProgressBar', () => {
  return function MockProgressBar() {
    return <div role="progressbar" />
  }
})

type UserBookOverride = {
  status?: string
  tags?: string[]
  formats?: string[]
  progressMode?: string
}

function makeUserBook(id: string, title: string, overrides: UserBookOverride = {}) {
  return create(UserBookSchema, {
    id,
    status: 'wishlist',
    tags: [],
    formats: [],
    progressMode: 'pages',
    book: create(BookSchema, { title, authors: ['Author'] }),
    ...overrides
  })
}

const readingBook = makeUserBook('1', 'Dune', { status: 'currently-reading' })
const wishlistBook = makeUserBook('2', 'Hyperion', { status: 'to-read' })
const finishedBook = makeUserBook('3', 'Foundation', { status: 'read' })
const shelfBook = makeUserBook('4', 'Neuromancer', { status: 'to-read' })
const physicalBook = makeUserBook('5', 'Physical Only', {
  status: 'currently-reading',
  tags: ['own-physical']
})
const pdfBook = makeUserBook('6', 'PDF Book', {
  status: 'currently-reading',
  formats: ['pdf']
})

type LibraryOverride = {
  reading?: ReturnType<typeof makeUserBook>[]
  wishlist?: ReturnType<typeof makeUserBook>[]
  finished?: ReturnType<typeof makeUserBook>[]
  shelves?: ReturnType<typeof create<typeof BookShelfSchema>>[]
}

function makeLibrary(overrides: LibraryOverride = {}) {
  return create(LibraryResponseSchema, {
    reading: [readingBook],
    wishlist: [wishlistBook],
    finished: [finishedBook],
    shelves: [create(BookShelfSchema, { name: 'Sci-Fi', books: [shelfBook] })],
    ...overrides
  })
}

describe('BooksLibrary', () => {
  it('defaults to the first non-empty shelf (Currently Reading)', () => {
    render(<BooksLibrary library={makeLibrary()} onEdit={jest.fn()} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.queryByText('Hyperion')).not.toBeInTheDocument()
  })

  it('switches shelf when sidebar nav is clicked', () => {
    render(<BooksLibrary library={makeLibrary()} onEdit={jest.fn()} />)
    // Click Wishlist in the sidebar (desktop nav)
    const wishlistBtns = screen.getAllByText('Wishlist')
    fireEvent.click(wishlistBtns[0])
    expect(screen.getByText('Hyperion')).toBeInTheDocument()
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
  })

  it('shows dynamic shelves in the sidebar', () => {
    render(<BooksLibrary library={makeLibrary()} onEdit={jest.fn()} />)
    expect(screen.getAllByText('Sci-Fi').length).toBeGreaterThan(0)
  })

  it('navigates to a dynamic shelf', () => {
    render(<BooksLibrary library={makeLibrary()} onEdit={jest.fn()} />)
    fireEvent.click(screen.getAllByText('Sci-Fi')[0])
    expect(screen.getByText('Neuromancer')).toBeInTheDocument()
  })

  it('shows shelf book count in sidebar', () => {
    render(<BooksLibrary library={makeLibrary()} onEdit={jest.fn()} />)
    // Each shelf shows a count - wishlist has 1 book
    const wishlistBtns = screen.getAllByText('Wishlist')
    const wishlistContainer = wishlistBtns[0].closest('button')
    expect(wishlistContainer?.textContent).toContain('1')
  })

  it('Physical filter narrows results', () => {
    const library = makeLibrary({ reading: [readingBook, physicalBook] })
    render(<BooksLibrary library={library} onEdit={jest.fn()} />)
    // Both books visible initially
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Physical Only')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Physical' }))
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
    expect(screen.getByText('Physical Only')).toBeInTheDocument()
  })

  it('PDF filter narrows results', () => {
    const library = makeLibrary({ reading: [readingBook, pdfBook] })
    render(<BooksLibrary library={library} onEdit={jest.fn()} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('PDF Book')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'PDF' }))
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
    expect(screen.getByText('PDF Book')).toBeInTheDocument()
  })

  it('combined AND filters narrow results', () => {
    const both = makeUserBook('7', 'Both', {
      status: 'currently-reading',
      tags: ['own-physical'],
      formats: ['pdf']
    })
    const library = makeLibrary({ reading: [readingBook, physicalBook, pdfBook, both] })
    render(<BooksLibrary library={library} onEdit={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Physical' }))
    fireEvent.click(screen.getByRole('button', { name: 'PDF' }))
    // Only "Both" passes both filters
    expect(screen.getByText('Both')).toBeInTheDocument()
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
    expect(screen.queryByText('Physical Only')).not.toBeInTheDocument()
    expect(screen.queryByText('PDF Book')).not.toBeInTheDocument()
  })

  it('Clear button removes all filters', () => {
    const library = makeLibrary({ reading: [readingBook, physicalBook] })
    render(<BooksLibrary library={library} onEdit={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Physical' }))
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Physical Only')).toBeInTheDocument()
  })

  it('shows empty message when filters match nothing', () => {
    render(<BooksLibrary library={makeLibrary()} onEdit={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'PDF' }))
    expect(screen.getByText('No books match the current filters.')).toBeInTheDocument()
  })

  it('resets page when switching shelf', () => {
    // Create 25 books so pagination triggers (PAGE_SIZE=20)
    const manyBooks = Array.from({ length: 25 }, (_, i) =>
      makeUserBook(`r${i}`, `Book ${i}`, { status: 'currently-reading' })
    )
    const library = makeLibrary({ reading: manyBooks })
    render(<BooksLibrary library={library} onEdit={jest.fn()} />)

    // Should show pagination
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('2 / 2')).toBeInTheDocument()

    // Switch shelf — page should reset
    fireEvent.click(screen.getAllByText('Wishlist')[0])
    fireEvent.click(screen.getAllByText('Currently Reading')[0])
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
  })

  it('shows prev/next pagination controls for large lists', () => {
    const manyBooks = Array.from({ length: 25 }, (_, i) =>
      makeUserBook(`p${i}`, `Book ${i}`, { status: 'currently-reading' })
    )
    render(<BooksLibrary library={makeLibrary({ reading: manyBooks })} onEdit={jest.fn()} />)
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Prev' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled()

    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('2 / 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
  })

  it('hides pagination when all books fit on one page', () => {
    render(<BooksLibrary library={makeLibrary()} onEdit={jest.fn()} />)
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument()
  })

  it('calls onEdit when Edit is clicked', () => {
    const onEdit = jest.fn()
    render(<BooksLibrary library={makeLibrary()} onEdit={onEdit} />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(onEdit).toHaveBeenCalledWith(readingBook)
  })
})
