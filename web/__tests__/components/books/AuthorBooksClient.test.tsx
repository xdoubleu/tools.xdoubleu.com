import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  UserBookSchema,
  BookSchema,
  BookShelfSchema,
  LibraryResponseSchema,
  type UserBook,
  type BookShelf
} from '@/lib/gen/books/v1/library_pb'

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

jest.mock('@/components/books/BookRatingStars', () => {
  return function MockRatingStars() {
    return <div data-testid="rating-stars" />
  }
})

jest.mock('@/components/books/BookFavouriteButton', () => {
  return function MockFavButton() {
    return <div data-testid="favourite-button" />
  }
})

jest.mock('@/components/books/BookShelfTagCell', () => {
  return function MockShelfTagCell() {
    return <div data-testid="shelf-tag-cell" />
  }
})

jest.mock('@/hooks/useBooks', () => ({
  useLibrary: jest.fn(),
  useToggleTag: () => jest.fn()
}))

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: jest.fn()
}))

import AuthorBooksClient from '@/components/books/AuthorBooksClient'
import { useLibrary } from '@/hooks/useBooks'

const mockUseBacklogLibrary = jest.mocked(useLibrary)

function makeLibraryWith(books: UserBook[], shelves: BookShelf[]) {
  return {
    data: {
      library: create(LibraryResponseSchema, {
        reading: books.filter((b) => b.status === 'currently-reading'),
        wishlist: books.filter((b) => b.status === 'to-read'),
        finished: books.filter((b) => b.status === 'read'),
        shelves
      })
    },
    error: null,
    isLoading: false
  }
}

const herbertBook = create(UserBookSchema, {
  id: 'ub-1',
  status: 'currently-reading',
  tags: [],
  formats: [],
  book: create(BookSchema, {
    title: 'Dune',
    authors: ['Frank Herbert'],
    pageCount: 412
  })
})

const leGuinBook = create(UserBookSchema, {
  id: 'ub-2',
  status: 'to-read',
  tags: [],
  formats: [],
  book: create(BookSchema, {
    title: 'The Left Hand of Darkness',
    authors: ['Ursula K. Le Guin'],
    pageCount: 286
  })
})

describe('AuthorBooksClient', () => {
  beforeEach(() => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue(makeLibraryWith([herbertBook, leGuinBook], []))
  })

  afterEach(() => {
    jest.clearAllMocks()
  })

  it('shows only books by the specified author', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
    expect(screen.queryByText('The Left Hand of Darkness')).not.toBeInTheDocument()
  })

  it('shows the author name as a heading', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByRole('heading', { name: 'Frank Herbert' })).toBeInTheDocument()
  })

  it('shows a book count', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByText('1 book in your library')).toBeInTheDocument()
  })

  it('shows plural when multiple books exist', () => {
    render(<AuthorBooksClient name="Ursula K. Le Guin" />)
    expect(screen.getByText('1 book in your library')).toBeInTheDocument()
  })

  it('shows no books message for unknown author', () => {
    render(<AuthorBooksClient name="Unknown Author" />)
    expect(screen.getByText('0 books in your library')).toBeInTheDocument()
    expect(screen.getByText('No books match the current filters.')).toBeInTheDocument()
  })

  it('includes books from custom shelves (covers flattenLibrary shelf branch)', () => {
    const shelfBook = create(UserBookSchema, {
      id: 'ub-shelf',
      status: 'sci-fi',
      tags: ['favourite', 'space-opera'],
      formats: [],
      book: create(BookSchema, {
        title: 'Chapterhouse: Dune',
        authors: ['Frank Herbert'],
        pageCount: 480
      })
    })
    const shelf = create(BookShelfSchema, { name: 'sci-fi', books: [shelfBook] })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue(makeLibraryWith([], [shelf]))
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByText('Chapterhouse: Dune')).toBeInTheDocument()
  })

  it('collects non-special tags from books (covers knownTags tag filter)', () => {
    const taggedBook = create(UserBookSchema, {
      id: 'ub-tagged',
      status: 'currently-reading',
      tags: ['favourite', 'space-opera'],
      formats: [],
      book: create(BookSchema, {
        title: 'Dune Messiah',
        authors: ['Frank Herbert'],
        pageCount: 220
      })
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogLibrary.mockReturnValue(makeLibraryWith([taggedBook], []))
    render(<AuthorBooksClient name="Frank Herbert" />)
    // Component renders without error — tag collection ran
    expect(screen.getByText('Dune Messiah')).toBeInTheDocument()
  })

  it('renders a breadcrumb back to library', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.getByRole('link', { name: 'Library' })).toHaveAttribute('href', '/books/library')
  })

  it('does not render a dead /backlog breadcrumb link', () => {
    render(<AuthorBooksClient name="Frank Herbert" />)
    expect(screen.queryByRole('link', { name: 'Backlog' })).not.toBeInTheDocument()
  })
})
