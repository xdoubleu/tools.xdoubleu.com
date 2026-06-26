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

  it('filters by tag when a tag is clicked, showing books from all shelves with that tag', () => {
    const taggedReading = makeUserBook('t1', 'Tagged Reading', {
      status: 'currently-reading',
      tags: ['fantasy']
    })
    const taggedFinished = makeUserBook('t2', 'Tagged Finished', {
      status: 'read',
      tags: ['fantasy']
    })
    const untagged = makeUserBook('t3', 'Untagged', { status: 'currently-reading' })
    const library = makeLibrary({ reading: [taggedReading, untagged], finished: [taggedFinished] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)

    fireEvent.click(screen.getAllByText('fantasy')[0])
    expect(screen.getByText('Tagged Reading')).toBeInTheDocument()
    expect(screen.getByText('Tagged Finished')).toBeInTheDocument()
    expect(screen.queryByText('Untagged')).not.toBeInTheDocument()
  })

  it('selecting a tag clears any shelf filter (exclusive selection)', () => {
    const taggedBook = makeUserBook('t1', 'Tagged', {
      status: 'currently-reading',
      tags: ['fantasy']
    })
    const wishlistTagged = makeUserBook('t2', 'Wishlist Tagged', {
      status: 'to-read',
      tags: ['fantasy']
    })
    const library = makeLibrary({ reading: [taggedBook], wishlist: [wishlistTagged] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)

    // Select a shelf first
    fireEvent.click(screen.getAllByText('Currently reading')[0])
    expect(screen.queryByText('Wishlist Tagged')).not.toBeInTheDocument()

    // Selecting a tag should show books from ALL shelves matching the tag
    fireEvent.click(screen.getAllByText('fantasy')[0])
    expect(screen.getByText('Tagged')).toBeInTheDocument()
    expect(screen.getByText('Wishlist Tagged')).toBeInTheDocument()
  })

  it('re-clicking the active tag returns to All books', () => {
    const taggedBook = makeUserBook('t1', 'Tagged', {
      status: 'currently-reading',
      tags: ['fantasy']
    })
    const untagged = makeUserBook('t2', 'Untagged', { status: 'currently-reading' })
    const library = makeLibrary({ reading: [taggedBook, untagged] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)

    const fantasyBtns = screen.getAllByText('fantasy')
    fireEvent.click(fantasyBtns[0]) // activate tag
    expect(screen.queryByText('Untagged')).not.toBeInTheDocument()

    fireEvent.click(screen.getAllByText('fantasy')[0]) // deactivate → back to all
    expect(screen.getByText('Tagged')).toBeInTheDocument()
    expect(screen.getByText('Untagged')).toBeInTheDocument()
  })

  it('shows only favourite books when Favourites shelf is selected', () => {
    const favReading = makeUserBook('fav1', 'Fav Reading', {
      status: 'currently-reading',
      tags: ['favourite']
    })
    const favFinished = makeUserBook('fav2', 'Fav Finished', {
      status: 'read',
      tags: ['favourite']
    })
    const notFav = makeUserBook('nf1', 'Not Fav', { status: 'currently-reading' })
    const library = makeLibrary({ reading: [favReading, notFav], finished: [favFinished] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)

    fireEvent.click(screen.getAllByText('Favourites')[0])
    expect(screen.getByText('Fav Reading')).toBeInTheDocument()
    expect(screen.getByText('Fav Finished')).toBeInTheDocument()
    expect(screen.queryByText('Not Fav')).not.toBeInTheDocument()
  })

  it('Favourites shelf cross-cuts all reading statuses', () => {
    const favWishlist = makeUserBook('fav3', 'Fav Wishlist', {
      status: 'to-read',
      tags: ['favourite']
    })
    const library = makeLibrary({ wishlist: [favWishlist] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)

    fireEvent.click(screen.getAllByText('Favourites')[0])
    expect(screen.getByText('Fav Wishlist')).toBeInTheDocument()
  })

  it('Favourites shelf is mutually exclusive with tag selection', () => {
    const favTagged = makeUserBook('ft1', 'Fav Tagged', {
      status: 'currently-reading',
      tags: ['favourite', 'fantasy']
    })
    const taggedOnly = makeUserBook('t1', 'Tagged Only', {
      status: 'currently-reading',
      tags: ['fantasy']
    })
    const library = makeLibrary({ reading: [favTagged, taggedOnly] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)

    // Select favourites shelf
    fireEvent.click(screen.getAllByText('Favourites')[0])
    expect(screen.getByText('Fav Tagged')).toBeInTheDocument()
    expect(screen.queryByText('Tagged Only')).not.toBeInTheDocument()

    // Selecting a tag clears the shelf and shows all books with that tag
    fireEvent.click(screen.getAllByText('fantasy')[0])
    expect(screen.getByText('Fav Tagged')).toBeInTheDocument()
    expect(screen.getByText('Tagged Only')).toBeInTheDocument()
  })

  it('does not default to Favourites shelf on initial render', () => {
    const favBook = makeUserBook('fav1', 'Fav Book', {
      status: 'currently-reading',
      tags: ['favourite']
    })
    const library = makeLibrary({ reading: [favBook] })
    render(<BooksLibrary library={library} knownShelves={[]} onSaved={jest.fn()} />)
    // Should default to currently-reading (not favourites)
    const header = screen.getByRole('heading')
    expect(header.textContent).not.toMatch(/Favourites/)
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
