import React from 'react'
import { render, screen, fireEvent, waitFor, act, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import BooksLibrary from '@/components/books/BooksLibrary'
import {
  UserBookSchema,
  BookSchema,
  BookShelfSchema,
  LibraryResponseSchema
} from '@/lib/gen/books/v1/library_pb'

const mockSearchExternal = vi.fn()
vi.mock('@/hooks/useBooks', async () => ({
  ...(await vi.importActual('@/hooks/useBooks')),
  useSearchExternal: () => mockSearchExternal
}))

vi.useFakeTimers()

vi.mock('@/components/books/BookRatingStars', () => {
  return {
    default: function MockRatingStars() {
      return <div data-testid="rating-stars" />
    }
  }
})

vi.mock('@/components/books/BookFavouriteButton', () => {
  return {
    default: function MockFavButton() {
      return <div data-testid="favourite-button" />
    }
  }
})

vi.mock('@/components/books/ManageShelvesTagsDialog', () => {
  return {
    default: function MockManageDialog({ open }: { open: boolean }) {
      return open ? <div data-testid="manage-dialog" /> : null
    }
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

function renderLibrary(
  library: ReturnType<typeof makeLibrary>,
  opts: { searchQuery?: string } = {}
) {
  return render(
    <BooksLibrary
      library={library}
      knownShelves={[]}
      searchQuery={opts.searchQuery ?? ''}
      onSearchChange={vi.fn()}
      onSaved={vi.fn()}
    />
  )
}

describe('BooksLibrary', () => {
  beforeEach(() => {
    // Most tests drive the table; the cards view has its own tests below.
    localStorage.setItem('books:library:view', JSON.stringify('table'))
    mockSearchExternal.mockReset()
    mockSearchExternal.mockResolvedValue({ results: [] })
  })

  afterEach(() => {
    vi.clearAllTimers()
    localStorage.clear()
  })

  describe('cards view', () => {
    beforeEach(() => localStorage.clear())

    it('shows book cards by default, without the table controls', () => {
      renderLibrary(makeLibrary())
      expect(screen.getByRole('tab', { name: 'Cards', selected: true })).toBeInTheDocument()
      expect(screen.queryByRole('table')).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Columns' })).not.toBeInTheDocument()
      expect(screen.getByRole('link', { name: 'Dune' })).toHaveAttribute('href', '/books/1')
    })

    it('switches to the table and remembers the choice on this device', () => {
      renderLibrary(makeLibrary())
      fireEvent.click(screen.getByRole('tab', { name: 'Table' }))
      expect(screen.getByRole('table')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Columns' })).toBeInTheDocument()
      expect(localStorage.getItem('books:library:view')).toBe('"table"')
    })

    it('counts the shelf and says so when it is empty', () => {
      renderLibrary(makeLibrary({ reading: [], wishlist: [], finished: [], shelves: [] }))
      expect(screen.getByText('Nothing on this shelf yet.')).toBeInTheDocument()
      expect(screen.getByText('0 books')).toBeInTheDocument()
    })

    it('hides the view switch while searching', () => {
      renderLibrary(makeLibrary(), { searchQuery: 'a' })
      expect(screen.queryByRole('tab', { name: 'Cards' })).not.toBeInTheDocument()
    })
  })

  it('defaults to the All books shelf', () => {
    renderLibrary(makeLibrary())
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('All books')
    expect(screen.getAllByText('Dune').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Hyperion').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Foundation').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Neuromancer').length).toBeGreaterThan(0)
  })

  it("links the library's series in the sidebar", () => {
    const inSeries = create(UserBookSchema, {
      id: '9',
      status: 'read',
      book: create(BookSchema, { title: 'Mort', authors: ['Author'], seriesName: 'Discworld' })
    })
    renderLibrary(makeLibrary({ finished: [inSeries] }))
    const links = screen.getAllByRole('link', { name: /^Discworld/ })
    expect(links.length).toBeGreaterThan(0)
    expect(links[0]).toHaveAttribute('href', '/books/series/Discworld')
  })

  it('switches to All books when clicked', () => {
    renderLibrary(makeLibrary())
    const allBtns = screen.getAllByText('All books')
    fireEvent.click(allBtns[0])
    expect(screen.getAllByText('Dune').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Hyperion').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Foundation').length).toBeGreaterThan(0)
  })

  it('switches shelf when sidebar nav is clicked', () => {
    renderLibrary(makeLibrary())
    const wantBtns = screen.getAllByText('Want to read')
    fireEvent.click(wantBtns[0])
    expect(screen.getAllByText('Hyperion').length).toBeGreaterThan(0)
    expect(screen.queryByText('Dune')).not.toBeInTheDocument()
  })

  it('shows dynamic shelves in the sidebar', () => {
    renderLibrary(makeLibrary())
    expect(screen.getAllByText('Sci-Fi').length).toBeGreaterThan(0)
  })

  it('navigates to a dynamic shelf', () => {
    renderLibrary(makeLibrary())
    fireEvent.click(screen.getAllByText('Sci-Fi')[0])
    expect(screen.getAllByText('Neuromancer').length).toBeGreaterThan(0)
  })

  it('opens manage dialog when Edit shelves & tags is clicked', () => {
    renderLibrary(makeLibrary())
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
    renderLibrary(library)

    fireEvent.click(screen.getAllByText('fantasy')[0])
    expect(screen.getAllByText('Tagged Reading').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Tagged Finished').length).toBeGreaterThan(0)
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
    renderLibrary(library)

    fireEvent.click(screen.getAllByText('Currently reading')[0])
    expect(screen.queryByText('Wishlist Tagged')).not.toBeInTheDocument()

    // A tag spans all shelves.
    fireEvent.click(screen.getAllByText('fantasy')[0])
    expect(screen.getAllByText('Tagged').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Wishlist Tagged').length).toBeGreaterThan(0)
  })

  it('re-clicking the active tag returns to All books', () => {
    const taggedBook = makeUserBook('t1', 'Tagged', {
      status: 'currently-reading',
      tags: ['fantasy']
    })
    const untagged = makeUserBook('t2', 'Untagged', { status: 'currently-reading' })
    const library = makeLibrary({ reading: [taggedBook, untagged] })
    renderLibrary(library)

    const fantasyBtns = screen.getAllByText('fantasy')
    fireEvent.click(fantasyBtns[0]) // activate tag
    expect(screen.queryByText('Untagged')).not.toBeInTheDocument()

    fireEvent.click(screen.getAllByText('fantasy')[0]) // deactivate → back to all
    expect(screen.getAllByText('Tagged').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Untagged').length).toBeGreaterThan(0)
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
    renderLibrary(library)

    fireEvent.click(screen.getAllByText('Favourites')[0])
    expect(screen.getAllByText('Fav Reading').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Fav Finished').length).toBeGreaterThan(0)
    expect(screen.queryByText('Not Fav')).not.toBeInTheDocument()
  })

  it('Favourites shelf cross-cuts all reading statuses', () => {
    const favWishlist = makeUserBook('fav3', 'Fav Wishlist', {
      status: 'to-read',
      tags: ['favourite']
    })
    const library = makeLibrary({ wishlist: [favWishlist] })
    renderLibrary(library)

    fireEvent.click(screen.getAllByText('Favourites')[0])
    expect(screen.getAllByText('Fav Wishlist').length).toBeGreaterThan(0)
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
    renderLibrary(library)

    fireEvent.click(screen.getAllByText('Favourites')[0])
    expect(screen.getAllByText('Fav Tagged').length).toBeGreaterThan(0)
    expect(screen.queryByText('Tagged Only')).not.toBeInTheDocument()

    fireEvent.click(screen.getAllByText('fantasy')[0])
    expect(screen.getAllByText('Fav Tagged').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Tagged Only').length).toBeGreaterThan(0)
  })

  it('does not default to Favourites shelf on initial render', () => {
    const favBook = makeUserBook('fav1', 'Fav Book', {
      status: 'currently-reading',
      tags: ['favourite']
    })
    const library = makeLibrary({ reading: [favBook] })
    renderLibrary(library)
    const header = screen.getByRole('heading', { level: 2 })
    expect(header.textContent).not.toMatch(/Favourites/)
  })

  it('shows empty message when library has no books', () => {
    const emptyLib = makeLibrary({ reading: [], wishlist: [], finished: [], shelves: [] })
    renderLibrary(emptyLib)
    expect(screen.getAllByText('No books match the current filters.').length).toBeGreaterThan(0)
  })

  it('shows pagination for large book lists', () => {
    const manyBooks = Array.from({ length: 25 }, (_, i) =>
      makeUserBook(`r${i}`, `Book ${i}`, { status: 'currently-reading' })
    )
    const library = makeLibrary({ reading: manyBooks })
    renderLibrary(library)
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled()
  })

  it('hides pagination when all books fit on one page', () => {
    renderLibrary(makeLibrary())
    expect(screen.queryByRole('button', { name: 'Next' })).not.toBeInTheDocument()
  })

  it('filters across all shelves by title when searchQuery is set', () => {
    renderLibrary(makeLibrary(), { searchQuery: 'dune' })
    expect(screen.getAllByText('Dune').length).toBeGreaterThan(0)
    expect(screen.queryByText('Hyperion')).not.toBeInTheDocument()
    expect(screen.queryByText('Foundation')).not.toBeInTheDocument()
  })

  it('filters by author substring when searchQuery is set', () => {
    const bookA = create(UserBookSchema, {
      id: 'a1',
      status: 'currently-reading',
      tags: [],
      formats: [],
      book: create(BookSchema, { title: 'Alpha', authors: ['Zelda Smith'] })
    })
    const bookB = create(UserBookSchema, {
      id: 'a2',
      status: 'currently-reading',
      tags: [],
      formats: [],
      book: create(BookSchema, { title: 'Beta', authors: ['John Doe'] })
    })
    const lib2 = create(LibraryResponseSchema, {
      reading: [bookA, bookB],
      wishlist: [],
      finished: [],
      shelves: []
    })
    renderLibrary(lib2, { searchQuery: 'zelda' })
    expect(screen.getByText('Alpha')).toBeInTheDocument()
    expect(screen.queryByText('Beta')).not.toBeInTheDocument()
  })

  it('shows header as "Search results" while searchQuery is active', () => {
    renderLibrary(makeLibrary(), { searchQuery: 'dune' })
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Search results')
  })

  it('carries the search query into result card links so the breadcrumb can restore it', () => {
    const { container } = renderLibrary(makeLibrary(), { searchQuery: 'dune' })
    const link = container.querySelector('a[href^="/books/1"]')
    expect(link).toHaveAttribute('href', '/books/1?q=dune')
  })

  it('does not fetch external results when the library already has a match', async () => {
    renderLibrary(makeLibrary(), { searchQuery: 'dune' })
    await act(async () => {
      vi.advanceTimersByTime(300)
    })
    expect(mockSearchExternal).not.toHaveBeenCalled()
  })

  it('shows a not-in-library card with its source when the query has no library match', async () => {
    mockSearchExternal.mockResolvedValue({
      results: [
        {
          provider: 'hardcover',
          providerId: '9780000000001',
          title: 'Far Away Book',
          authors: ['Someone Else'],
          isbn13: '',
          coverUrl: '',
          description: ''
        }
      ]
    })
    renderLibrary(makeLibrary(), { searchQuery: 'not-in-library' })

    await act(async () => {
      vi.advanceTimersByTime(300)
    })
    await waitFor(() => {
      expect(mockSearchExternal).toHaveBeenCalledWith('not-in-library')
      expect(screen.getByText('Far Away Book')).toBeInTheDocument()
    })
    expect(screen.getByText('Hardcover')).toBeInTheDocument()
    expect(screen.getByText('Far Away Book').closest('a')).toHaveAttribute(
      'href',
      '/books/external/hardcover/9780000000001'
    )
  })

  it('shows "No results." when neither the library nor the external search match', async () => {
    renderLibrary(makeLibrary(), { searchQuery: 'nothing-matches-anything' })
    await act(async () => {
      vi.advanceTimersByTime(300)
    })
    await waitFor(() => {
      expect(screen.getByText('No results.')).toBeInTheDocument()
    })
  })

  it('puts search, filters and the result count in one toolbar', () => {
    renderLibrary(makeLibrary())
    const toolbar = screen.getByRole('toolbar', { name: 'Filter books' })
    expect(within(toolbar).getByRole('searchbox', { name: 'Search books' })).toBeInTheDocument()
    expect(within(toolbar).getByRole('button', { name: 'Filters' })).toBeInTheDocument()
    expect(within(toolbar).getByRole('button', { name: 'Columns' })).toBeInTheDocument()
    expect(within(toolbar).getByText('4 books')).toBeInTheDocument()
  })

  it('counts only the books the table filters leave', () => {
    localStorage.setItem('backlog:library:filter:kobo', JSON.stringify(['kobo-sync']))
    renderLibrary(makeLibrary())
    expect(screen.getByText('0 books')).toBeInTheDocument()
    localStorage.clear()
  })

  it('drops the table filters while searching', () => {
    renderLibrary(makeLibrary(), { searchQuery: 'Dune' })
    const toolbar = screen.getByRole('toolbar', { name: 'Filter books' })
    expect(within(toolbar).queryByRole('button', { name: 'Filters' })).not.toBeInTheDocument()
    expect(within(toolbar).getByText('1 book')).toBeInTheDocument()
  })
})
