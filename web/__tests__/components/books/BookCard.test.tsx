import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import BookCard from '@/components/books/BookCard'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

jest.mock('next/link', () => {
  return ({
    children,
    href,
    ...props
  }: {
    children: React.ReactNode
    href: string
    [key: string]: unknown
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  )
})

jest.mock('next/image', () => {
  return function MockImage({ src, alt, ...props }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} {...props} />
  }
})

jest.mock('@/components/books/BookProgressEditor', () => {
  return function MockProgressEditor() {
    return <div role="progressbar" data-testid="progress-editor" />
  }
})

jest.mock('@/components/books/BookRatingStars', () => {
  return function MockRatingStars({ userBook }: { userBook: { rating: number } }) {
    return <div data-testid="rating-stars">{userBook.rating} stars</div>
  }
})

jest.mock('@/components/books/BookFavouriteButton', () => {
  return function MockFavButton() {
    return <div data-testid="favourite-button" />
  }
})

jest.mock('@/components/books/BookOwnershipToggles', () => {
  return function MockOwnership({ userBook }: { userBook: { tags: string[]; formats: string[] } }) {
    return (
      <div data-testid="ownership-toggles">
        {userBook.tags.includes('own-physical') && <span>Physical</span>}
        {userBook.tags.includes('own-digital') && <span>Digital</span>}
        {userBook.formats.includes('pdf') && <span>PDF</span>}
        {userBook.formats.includes('epub') && <span>EPUB</span>}
      </div>
    )
  }
})

type BookOverride = {
  status?: string
  tags?: string[]
  formats?: string[]
  progressMode?: string
  currentPage?: number
  progressPercent?: number
  rating?: number
  book?: ReturnType<typeof create<typeof BookSchema>>
}

function makeBook(overrides: BookOverride = {}) {
  return create(UserBookSchema, {
    id: 'ub-1',
    status: 'to-read',
    tags: [],
    formats: [],
    progressMode: 'pages',
    book: create(BookSchema, {
      title: 'Test Book',
      authors: ['Test Author']
    }),
    ...overrides
  })
}

describe('BookCard', () => {
  it('renders title and author', () => {
    render(<BookCard userBook={makeBook()} onSaved={jest.fn()} />)
    expect(screen.getByText('Test Book')).toBeInTheDocument()
    expect(screen.getByText('Test Author')).toBeInTheDocument()
  })

  it('renders a link to the book detail page', () => {
    render(<BookCard userBook={makeBook()} onSaved={jest.fn()} />)
    const link = screen.getByRole('link', { name: 'Test Book' })
    expect(link).toHaveAttribute('href', '/books/ub-1')
  })

  it('carries the search query into the detail link when provided', () => {
    render(<BookCard userBook={makeBook()} onSaved={jest.fn()} query="dune" />)
    const link = screen.getByRole('link', { name: 'Test Book' })
    expect(link).toHaveAttribute('href', '/books/ub-1?q=dune')
  })

  it('shows Physical badge when own-physical tag present', () => {
    render(<BookCard userBook={makeBook({ tags: ['own-physical'] })} onSaved={jest.fn()} />)
    expect(screen.getByText('Physical')).toBeInTheDocument()
  })

  it('shows Digital badge when own-digital tag present', () => {
    render(<BookCard userBook={makeBook({ tags: ['own-digital'] })} onSaved={jest.fn()} />)
    expect(screen.getByText('Digital')).toBeInTheDocument()
  })

  it('shows PDF badge when formats includes pdf', () => {
    render(<BookCard userBook={makeBook({ formats: ['pdf'] })} onSaved={jest.fn()} />)
    expect(screen.getByText('PDF')).toBeInTheDocument()
  })

  it('shows EPUB badge when formats includes epub', () => {
    render(<BookCard userBook={makeBook({ formats: ['epub'] })} onSaved={jest.fn()} />)
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('shows progress editor for currently-reading books', () => {
    render(<BookCard userBook={makeBook({ status: 'currently-reading' })} onSaved={jest.fn()} />)
    expect(screen.getByTestId('progress-editor')).toBeInTheDocument()
  })

  it('hides progress editor for non-reading books', () => {
    render(<BookCard userBook={makeBook({ status: 'to-read' })} onSaved={jest.fn()} />)
    expect(screen.queryByTestId('progress-editor')).not.toBeInTheDocument()
  })

  it('renders cover image when coverUrl present', () => {
    render(
      <BookCard
        userBook={makeBook({
          book: create(BookSchema, {
            title: 'Test Book',
            authors: ['Test Author'],
            coverUrl: 'http://example.com/cover.png'
          })
        })}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByAltText('Test Book')).toBeInTheDocument()
  })

  it('renders rating stars and favourite for a read book', () => {
    render(<BookCard userBook={makeBook({ status: 'read', rating: 3 })} onSaved={jest.fn()} />)
    expect(screen.getByTestId('rating-stars')).toBeInTheDocument()
    expect(screen.getByTestId('favourite-button')).toBeInTheDocument()
  })

  it('hides rating stars and favourite for a to-read book', () => {
    render(<BookCard userBook={makeBook({ status: 'to-read' })} onSaved={jest.fn()} />)
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
  })

  it('hides rating stars and favourite for a currently-reading book', () => {
    render(<BookCard userBook={makeBook({ status: 'currently-reading' })} onSaved={jest.fn()} />)
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
  })

  it('hides rating stars and favourite for a dropped book', () => {
    render(<BookCard userBook={makeBook({ status: 'dropped' })} onSaved={jest.fn()} />)
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
  })

  it('renders the ownership toggles', () => {
    render(<BookCard userBook={makeBook()} onSaved={jest.fn()} />)
    expect(screen.getByTestId('ownership-toggles')).toBeInTheDocument()
  })

  it('returns null when book is missing', () => {
    const ub = create(UserBookSchema, { id: 'ub-nobook', status: 'to-read', tags: [], formats: [] })
    const { container } = render(<BookCard userBook={ub} onSaved={jest.fn()} />)
    expect(container.firstChild).toBeNull()
  })

  it('shows status text for non-reading books', () => {
    render(<BookCard userBook={makeBook({ status: 'to-read' })} onSaved={jest.fn()} />)
    expect(screen.getByText('to read')).toBeInTheDocument()
  })

  it('shows status text for currently-reading books', () => {
    render(<BookCard userBook={makeBook({ status: 'currently-reading' })} onSaved={jest.fn()} />)
    expect(screen.getByText('currently reading')).toBeInTheDocument()
  })

  it('shows read-only tags', () => {
    render(<BookCard userBook={makeBook({ tags: ['sci-fi'] })} onSaved={jest.fn()} />)
    expect(screen.getByText('sci-fi')).toBeInTheDocument()
  })

  it('all badges shown together', () => {
    render(
      <BookCard
        userBook={makeBook({ tags: ['own-physical', 'own-digital'], formats: ['pdf', 'epub'] })}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('Physical')).toBeInTheDocument()
    expect(screen.getByText('Digital')).toBeInTheDocument()
    expect(screen.getByText('PDF')).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('clicking ownership toggles does not throw', () => {
    render(<BookCard userBook={makeBook()} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByTestId('ownership-toggles'))
  })

  it('clicking progress editor does not throw', () => {
    render(<BookCard userBook={makeBook({ status: 'currently-reading' })} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByTestId('progress-editor'))
  })
})
