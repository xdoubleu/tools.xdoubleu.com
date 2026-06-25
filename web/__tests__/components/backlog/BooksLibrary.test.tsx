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
  return function MockRatingStars() {
    return <div data-testid="rating-stars" />
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

jest.mock('@/components/backlog/ManageShelvesTagsDialog', () => {
  return function MockManageDialog({ open }: { open: boolean }) {
    return open ? <div data-testid="manage-dialog" /> : null
  }
})

type UserBookOverride = {
  status?: string
  tags?: string[]
  formats?: string[]
  rating?: number
  addedAt?: string
  finishedAt?: string[]
}

function makeUserBook(id: string, title: string, overrides: UserBookOverride = {}) {
  return create(UserBookSchema, {
    id,
    status: 'to-read',
    tags: [],
    formats: [],
    book: create(BookSchema, { title, authors: ['Author'] }),
    ...overrides
  })
}

const readingBook = makeUserBook('1', 'Dune', { status: 'currently-reading' })
const wishlistBook = makeUserBook('2', 'Hyperion', { status: 'to-read' })
const finishedBook = makeUserBook('3', 'Foundation', { status: 'read' })
const shelfBook = makeUserBook('4', 'Neuromancer', { status: 'to-read' })

function makeLibrary(
  overrides: {
    reading?: ReturnType<typeof makeUserBook>[]
    wishlist?: ReturnType<typeof makeUserBook>[]
    finished?: ReturnType<typeof makeUserBook>[]
    shelves?: ReturnType<typeof create<typeof BookShelfSchema>>[]
  } = {}
) {
  return create(LibraryResponseSchema, {
    reading: [readingBook],
    wishlist: [wishlistBook],
    finished: [finishedBook],
    shelves: [create(BookShelfSchema, { name: 'Sci-Fi', books: [shelfBook] })],
    ...overrides
  })
}

describe('BooksLibrary', () => {
  it('defaults to the first non-empty shelf', () => {
    render(<BooksLibrary library={makeLibrary()} knownShelves={[]} onSaved={jest.fn()} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('switches to All books when clicked', () => {
    render(<BooksLibrary library={makeLibrary()} knownShelves={[]} onSaved={jest.fn()} />)
    const allBtns = screen.getAllByText('All books')
    fireEvent.click(allBtns[0])
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.getByText('Hyperion')).toBeInTheDocument()
    expect(screen.getByText('Foundation')).toBeInTheDocument()
  })

  it('switches shelf when sidebar nav is clicked', () => {
    render(<BooksLibrary library={makeLibrary()} knownShelves={[]} onSaved={jest.fn()} />)
    const wantBtns = screen.getAllByText('Want to read')
    fireEvent.click(wantBtns[0])
    expect(screen.getByText('Hyperion')).toBeInTheDocument()
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
  })

  it('shows dynamic shelves in the sidebar', () => {
    render(<BooksLibrary library={makeLibrary()} knownShelves={[]} onSaved={jest.fn()} />)
    expect(screen.getAllByText('Sci-Fi').length).toBeGreaterThan(0)
  })

  it('navigates to a dynamic shelf', () => {
    render(<BooksLibrary library={makeLibrary()} knownShelves={[]} onSaved={jest.fn()} />)
    fireEvent.click(screen.getAllByText('Sci-Fi')[0])
    expect(screen.getByText('Neuromancer')).toBeInTheDocument()
  })

  it('opens manage dialog when Edit shelves & tags is clicked', () => {
    render(<BooksLibrary library={makeLibrary()} knownShelves={[]} onSaved={jest.fn()} />)
    expect(screen.queryByTestId('manage-dialog')).not.toBeInTheDocument()
    fireEvent.click(screen.getByText('Edit shelves & tags'))
    expect(screen.getByTestId('manage-dialog')).toBeInTheDocument()
  })

  it('filters by tag when a tag is clicked in the sidebar', () => {
    const taggedBook = makeUserBook('t1', 'Tagged', {
      status: 'currently-reading',
      tags: ['fantasy']
    })
    const untagged = makeUserBook('t2', 'Untagged', { status: 'currently-reading' })
    const library = makeLibrary({ reading: [taggedBook, untagged] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)

    // The sidebar renders 'fantasy' in both desktop nav and mobile chip row;
    // click the first occurrence (desktop nav button).
    fireEvent.click(screen.getAllByText('fantasy')[0])
    expect(screen.getByText('Tagged')).toBeInTheDocument()
    expect(screen.queryByText('Untagged')).not.toBeInTheDocument()
  })

  it('shows empty message when library has no books', () => {
    const emptyLib = makeLibrary({ reading: [], wishlist: [], finished: [], shelves: [] })
    render(<BooksLibrary library={emptyLib} knownShelves={[]} onSaved={jest.fn()} />)
    expect(screen.getByText('No books match the current filters.')).toBeInTheDocument()
  })

  it('shows pagination for large book lists', () => {
    const manyBooks = Array.from({ length: 25 }, (_, i) =>
      makeUserBook(`r${i}`, `Book ${i}`, { status: 'currently-reading' })
    )
    const library = makeLibrary({ reading: manyBooks })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled()
  })

  it('hides pagination when all books fit on one page', () => {
    render(<BooksLibrary library={makeLibrary()} knownShelves={[]} onSaved={jest.fn()} />)
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument()
  })
})
