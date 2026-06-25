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

jest.mock('@/components/backlog/BookProgressBar', () => {
  return function MockProgressBar() {
    return <div role="progressbar" />
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
    status: 'wishlist',
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
    render(<BookCard userBook={makeBook()} onAction={jest.fn()} />)
    expect(screen.getByText('Test Book')).toBeInTheDocument()
    expect(screen.getByText('Test Author')).toBeInTheDocument()
  })

  it('renders a link to the book detail page', () => {
    render(<BookCard userBook={makeBook()} onAction={jest.fn()} />)
    const link = screen.getByRole('link', { name: 'Test Book' })
    expect(link).toHaveAttribute('href', '/backlog/books/ub-1')
  })

  it('calls onAction with "entry" when Entry clicked', () => {
    const onAction = jest.fn()
    const ub = makeBook()
    render(<BookCard userBook={ub} onAction={onAction} />)
    fireEvent.click(screen.getByRole('button', { name: 'Entry' }))
    expect(onAction).toHaveBeenCalledWith('entry', ub)
  })

  it('calls onAction with "shelf" when Shelf clicked', () => {
    const onAction = jest.fn()
    const ub = makeBook()
    render(<BookCard userBook={ub} onAction={onAction} />)
    fireEvent.click(screen.getByRole('button', { name: 'Shelf' }))
    expect(onAction).toHaveBeenCalledWith('shelf', ub)
  })

  it('calls onAction with "progress" when Progress clicked', () => {
    const onAction = jest.fn()
    const ub = makeBook()
    render(<BookCard userBook={ub} onAction={onAction} />)
    fireEvent.click(screen.getByRole('button', { name: 'Progress' }))
    expect(onAction).toHaveBeenCalledWith('progress', ub)
  })

  it('shows Physical badge when own-physical tag present', () => {
    render(<BookCard userBook={makeBook({ tags: ['own-physical'] })} onAction={jest.fn()} />)
    expect(screen.getByText('Physical')).toBeInTheDocument()
  })

  it('shows Digital badge when own-digital tag present', () => {
    render(<BookCard userBook={makeBook({ tags: ['own-digital'] })} onAction={jest.fn()} />)
    expect(screen.getByText('Digital')).toBeInTheDocument()
  })

  it('shows PDF badge when formats includes pdf', () => {
    render(<BookCard userBook={makeBook({ formats: ['pdf'] })} onAction={jest.fn()} />)
    expect(screen.getByText('PDF')).toBeInTheDocument()
  })

  it('shows EPUB badge when formats includes epub', () => {
    render(<BookCard userBook={makeBook({ formats: ['epub'] })} onAction={jest.fn()} />)
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('shows all badges when all ownership/format conditions met', () => {
    render(
      <BookCard
        userBook={makeBook({ tags: ['own-physical', 'own-digital'], formats: ['pdf', 'epub'] })}
        onAction={jest.fn()}
      />
    )
    expect(screen.getByText('Physical')).toBeInTheDocument()
    expect(screen.getByText('Digital')).toBeInTheDocument()
    expect(screen.getByText('PDF')).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('hides badges section when no ownership or format tags', () => {
    render(<BookCard userBook={makeBook()} onAction={jest.fn()} />)
    expect(screen.queryByText('Physical')).not.toBeInTheDocument()
    expect(screen.queryByText('Digital')).not.toBeInTheDocument()
    expect(screen.queryByText('PDF')).not.toBeInTheDocument()
    expect(screen.queryByText('EPUB')).not.toBeInTheDocument()
  })

  it('shows progress bar for currently-reading books', () => {
    render(<BookCard userBook={makeBook({ status: 'currently-reading' })} onAction={jest.fn()} />)
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
  })

  it('hides progress bar for non-reading books', () => {
    render(<BookCard userBook={makeBook({ status: 'wishlist' })} onAction={jest.fn()} />)
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
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
        onAction={jest.fn()}
      />
    )
    expect(screen.getByAltText('Test Book')).toBeInTheDocument()
  })

  it('shows favourite indicator when favourite tag present', () => {
    render(<BookCard userBook={makeBook({ tags: ['favourite'] })} onAction={jest.fn()} />)
    expect(screen.getByText('♥')).toBeInTheDocument()
  })

  it('shows rating when rating > 0', () => {
    render(<BookCard userBook={makeBook({ rating: 4 })} onAction={jest.fn()} />)
    expect(screen.getByText('4★')).toBeInTheDocument()
  })
})
