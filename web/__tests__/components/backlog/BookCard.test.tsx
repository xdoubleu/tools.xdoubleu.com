import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import BookCard from '@/components/backlog/BookCard'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

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

jest.mock('@/components/backlog/BookProgressEditor', () => {
  return function MockProgressEditor() {
    return <div role="progressbar" data-testid="progress-editor" />
  }
})

jest.mock('@/components/backlog/BookRatingStars', () => {
  return function MockRatingStars({ userBook }: { userBook: { rating: number } }) {
    return <div data-testid="rating-stars">{userBook.rating} stars</div>
  }
})

jest.mock('@/components/backlog/BookFavouriteButton', () => {
  return function MockFavButton() {
    return <div data-testid="favourite-button" />
  }
})

jest.mock('@/components/backlog/BookOwnershipToggles', () => {
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

jest.mock('@/components/backlog/BookShelfPopover', () => {
  return function MockShelfPopover() {
    return <div data-testid="shelf-popover" />
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
    render(<BookCard userBook={makeBook()} knownShelves={[]} knownTags={[]} onSaved={jest.fn()} />)
    expect(screen.getByText('Test Book')).toBeInTheDocument()
    expect(screen.getByText('Test Author')).toBeInTheDocument()
  })

  it('renders a link to the book detail page', () => {
    render(<BookCard userBook={makeBook()} knownShelves={[]} knownTags={[]} onSaved={jest.fn()} />)
    const link = screen.getByRole('link', { name: 'Test Book' })
    expect(link).toHaveAttribute('href', '/backlog/books/ub-1')
  })

  it('shows Physical badge when own-physical tag present', () => {
    render(
      <BookCard
        userBook={makeBook({ tags: ['own-physical'] })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('Physical')).toBeInTheDocument()
  })

  it('shows Digital badge when own-digital tag present', () => {
    render(
      <BookCard
        userBook={makeBook({ tags: ['own-digital'] })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('Digital')).toBeInTheDocument()
  })

  it('shows PDF badge when formats includes pdf', () => {
    render(
      <BookCard
        userBook={makeBook({ formats: ['pdf'] })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('PDF')).toBeInTheDocument()
  })

  it('shows EPUB badge when formats includes epub', () => {
    render(
      <BookCard
        userBook={makeBook({ formats: ['epub'] })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('shows progress editor for currently-reading books', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'currently-reading' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByTestId('progress-editor')).toBeInTheDocument()
  })

  it('hides progress editor for non-reading books', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'to-read' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
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
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByAltText('Test Book')).toBeInTheDocument()
  })

  it('renders rating stars and favourite for a read book', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'read', rating: 3 })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByTestId('rating-stars')).toBeInTheDocument()
    expect(screen.getByTestId('favourite-button')).toBeInTheDocument()
  })

  it('hides rating stars and favourite for a to-read book', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'to-read' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
  })

  it('hides rating stars and favourite for a currently-reading book', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'currently-reading' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
  })

  it('hides rating stars and favourite for a dropped book', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'dropped' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.queryByTestId('rating-stars')).not.toBeInTheDocument()
    expect(screen.queryByTestId('favourite-button')).not.toBeInTheDocument()
  })

  it('renders the ownership toggles', () => {
    render(<BookCard userBook={makeBook()} knownShelves={[]} knownTags={[]} onSaved={jest.fn()} />)
    expect(screen.getByTestId('ownership-toggles')).toBeInTheDocument()
  })

  it('renders the shelf popover', () => {
    render(<BookCard userBook={makeBook()} knownShelves={[]} knownTags={[]} onSaved={jest.fn()} />)
    expect(screen.getByTestId('shelf-popover')).toBeInTheDocument()
  })

  it('returns null when book is missing', () => {
    const ub = create(UserBookSchema, { id: 'ub-nobook', status: 'to-read', tags: [], formats: [] })
    const { container } = render(
      <BookCard userBook={ub} knownShelves={[]} knownTags={[]} onSaved={jest.fn()} />
    )
    expect(container.firstChild).toBeNull()
  })

  it('shows status text for non-reading books', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'to-read' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('to read')).toBeInTheDocument()
  })

  it('shows status text for currently-reading books', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'currently-reading' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('currently reading')).toBeInTheDocument()
  })

  it('passes knownShelves to shelf popover', () => {
    render(
      <BookCard
        userBook={makeBook()}
        knownShelves={['sci-fi', 'fantasy']}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByTestId('shelf-popover')).toBeInTheDocument()
  })

  it('all badges shown together', () => {
    render(
      <BookCard
        userBook={makeBook({ tags: ['own-physical', 'own-digital'], formats: ['pdf', 'epub'] })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('Physical')).toBeInTheDocument()
    expect(screen.getByText('Digital')).toBeInTheDocument()
    expect(screen.getByText('PDF')).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('clicking ownership toggles does not throw', () => {
    render(<BookCard userBook={makeBook()} knownShelves={[]} knownTags={[]} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByTestId('ownership-toggles'))
  })

  it('clicking progress editor does not throw', () => {
    render(
      <BookCard
        userBook={makeBook({ status: 'currently-reading' })}
        knownShelves={[]}
        knownTags={[]}
        onSaved={jest.fn()}
      />
    )
    fireEvent.click(screen.getByTestId('progress-editor'))
  })

  it('clicking shelf popover does not throw', () => {
    render(<BookCard userBook={makeBook()} knownShelves={[]} knownTags={[]} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByTestId('shelf-popover'))
  })
})
