import React from 'react'
import { render, screen } from '@testing-library/react'
import DuplicateBookSummary from '@/components/backlog/DuplicateBookSummary'
import type { DupUserBook } from '@/components/backlog/DuplicateBookSummary'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeUB(
  overrides: {
    isbn13?: string
    coverUrl?: string
    description?: string
    pageCount?: number
    formats?: string[]
    tags?: string[]
    status?: string
    authors?: string[]
  } = {}
): DupUserBook {
  return {
    book: {
      id: 'book-1',
      title: 'Test Book',
      authors: overrides.authors ?? ['Test Author'],
      isbn13: overrides.isbn13 ?? '',
      coverUrl: overrides.coverUrl ?? '',
      description: overrides.description ?? '',
      pageCount: overrides.pageCount ?? 0
    },
    status: overrides.status ?? 'to-read',
    tags: overrides.tags ?? [],
    formats: overrides.formats ?? []
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('DuplicateBookSummary', () => {
  it('renders title and authors', () => {
    render(<DuplicateBookSummary ub={makeUB()} />)
    expect(screen.getByText('Test Book')).toBeInTheDocument()
    expect(screen.getByText('Test Author')).toBeInTheDocument()
  })

  // --- ISBN prefix (Part A) ---

  it('shows "ISBN <number>" when ISBN-13 is present', () => {
    render(<DuplicateBookSummary ub={makeUB({ isbn13: '9780261102217' })} />)
    expect(screen.getByText(/ISBN 9780261102217/)).toBeInTheDocument()
  })

  it('shows "No ISBN" when isbn13 is absent', () => {
    render(<DuplicateBookSummary ub={makeUB()} />)
    expect(screen.getByText(/No ISBN/)).toBeInTheDocument()
  })

  it('shows page count when present', () => {
    render(<DuplicateBookSummary ub={makeUB({ pageCount: 320 })} />)
    expect(screen.getByText(/320p/)).toBeInTheDocument()
  })

  it('does not show page count when zero', () => {
    render(<DuplicateBookSummary ub={makeUB({ pageCount: 0 })} />)
    expect(screen.queryByText(/\dp$/)).not.toBeInTheDocument()
  })

  // --- Metadata quality breakdown (Part B) ---

  it('shows Metadata score as X/5', () => {
    render(
      <DuplicateBookSummary
        ub={makeUB({
          isbn13: '9780261102217',
          pageCount: 300,
          coverUrl: 'https://example.com/c.jpg',
          description: 'A great book.'
        })}
      />
    )
    // authors + isbn13 + cover + desc + pageCount = 5/5
    expect(screen.getByText('Metadata 5/5')).toBeInTheDocument()
  })

  it('shows Metadata 0/5 when no fields are populated', () => {
    render(<DuplicateBookSummary ub={makeUB({ authors: [] })} />)
    expect(screen.getByText('Metadata 0/5')).toBeInTheDocument()
  })

  it('shows Cover badge when coverUrl is set', () => {
    render(<DuplicateBookSummary ub={makeUB({ coverUrl: 'https://example.com/c.jpg' })} />)
    expect(screen.getByText('Cover')).toBeInTheDocument()
  })

  it('shows "No cover" badge when coverUrl is empty', () => {
    render(<DuplicateBookSummary ub={makeUB({ coverUrl: '' })} />)
    expect(screen.getByText('No cover')).toBeInTheDocument()
  })

  it('shows Description badge when description is set', () => {
    render(<DuplicateBookSummary ub={makeUB({ description: 'Epic.' })} />)
    expect(screen.getByText('Description')).toBeInTheDocument()
  })

  it('shows "No description" badge when description is empty', () => {
    render(<DuplicateBookSummary ub={makeUB({ description: '' })} />)
    expect(screen.getByText('No description')).toBeInTheDocument()
  })

  it('shows Authors badge when authors are present', () => {
    render(<DuplicateBookSummary ub={makeUB({ authors: ['J.R.R. Tolkien'] })} />)
    expect(screen.getByText('Authors')).toBeInTheDocument()
  })

  it('shows "No authors" badge when authors array is empty', () => {
    render(<DuplicateBookSummary ub={makeUB({ authors: [] })} />)
    expect(screen.getByText('No authors')).toBeInTheDocument()
  })

  // --- Format / ownership badges ---

  it('renders PDF badge', () => {
    render(<DuplicateBookSummary ub={makeUB({ formats: ['pdf'] })} />)
    expect(screen.getByText('PDF')).toBeInTheDocument()
  })

  it('renders EPUB badge', () => {
    render(<DuplicateBookSummary ub={makeUB({ formats: ['epub'] })} />)
    expect(screen.getByText('EPUB')).toBeInTheDocument()
  })

  it('renders KEPUB badge', () => {
    render(<DuplicateBookSummary ub={makeUB({ formats: ['kepub'] })} />)
    expect(screen.getByText('KEPUB')).toBeInTheDocument()
  })

  it('renders Physical badge when own-physical tag is set', () => {
    render(<DuplicateBookSummary ub={makeUB({ tags: ['own-physical'] })} />)
    expect(screen.getByText('Physical')).toBeInTheDocument()
  })

  it('renders Digital badge when own-digital tag is set', () => {
    render(<DuplicateBookSummary ub={makeUB({ tags: ['own-digital'] })} />)
    expect(screen.getByText('Digital')).toBeInTheDocument()
  })

  it('renders status badge', () => {
    render(<DuplicateBookSummary ub={makeUB({ status: 'currently-reading' })} />)
    expect(screen.getByText('currently-reading')).toBeInTheDocument()
  })

  it('renders all present fields in a fully-populated entry', () => {
    render(
      <DuplicateBookSummary
        ub={makeUB({
          isbn13: '9780261102217',
          pageCount: 310,
          coverUrl: 'https://example.com/c.jpg',
          description: 'Epic fantasy.',
          formats: ['epub', 'kepub'],
          tags: ['own-physical'],
          status: 'read'
        })}
      />
    )
    expect(screen.getByText(/ISBN 9780261102217/)).toBeInTheDocument()
    expect(screen.getByText(/310p/)).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
    expect(screen.getByText('KEPUB')).toBeInTheDocument()
    expect(screen.getByText('Physical')).toBeInTheDocument()
    expect(screen.getByText('read')).toBeInTheDocument()
    // Quality badges
    expect(screen.getByText('Cover')).toBeInTheDocument()
    expect(screen.getByText('Description')).toBeInTheDocument()
    expect(screen.getByText('Authors')).toBeInTheDocument()
  })
})
