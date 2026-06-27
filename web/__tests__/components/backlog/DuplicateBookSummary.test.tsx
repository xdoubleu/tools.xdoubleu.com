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
    isbn10?: string
    coverUrl?: string
    description?: string
    pageCount?: number
    externalRefs?: Record<string, string>
    formats?: string[]
    tags?: string[]
    status?: string
  } = {}
): DupUserBook {
  return {
    book: {
      id: 'book-1',
      title: 'Test Book',
      authors: ['Test Author'],
      isbn13: overrides.isbn13 ?? '',
      isbn10: overrides.isbn10 ?? '',
      coverUrl: overrides.coverUrl ?? '',
      description: overrides.description ?? '',
      pageCount: overrides.pageCount ?? 0,
      externalRefs: overrides.externalRefs ?? {}
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

  it('shows ISBN-13 when present', () => {
    render(<DuplicateBookSummary ub={makeUB({ isbn13: '9780261102217' })} />)
    expect(screen.getByText(/9780261102217/)).toBeInTheDocument()
  })

  it('shows ISBN-10 when isbn13 is absent', () => {
    render(<DuplicateBookSummary ub={makeUB({ isbn10: '0261102214' })} />)
    expect(screen.getByText(/0261102214/)).toBeInTheDocument()
  })

  it('shows "No ISBN" when both are absent', () => {
    render(<DuplicateBookSummary ub={makeUB()} />)
    expect(screen.getByText(/No ISBN/)).toBeInTheDocument()
  })

  it('prefers ISBN-13 over ISBN-10', () => {
    render(<DuplicateBookSummary ub={makeUB({ isbn13: '9780261102217', isbn10: '0261102214' })} />)
    expect(screen.getByText(/9780261102217/)).toBeInTheDocument()
    expect(screen.queryByText(/0261102214/)).not.toBeInTheDocument()
  })

  it('shows page count when present', () => {
    render(<DuplicateBookSummary ub={makeUB({ pageCount: 320 })} />)
    expect(screen.getByText(/320p/)).toBeInTheDocument()
  })

  it('does not show page count when zero', () => {
    render(<DuplicateBookSummary ub={makeUB({ pageCount: 0 })} />)
    expect(screen.queryByText(/p$/)).not.toBeInTheDocument()
  })

  it('shows Cover + when cover URL is set', () => {
    render(<DuplicateBookSummary ub={makeUB({ coverUrl: 'https://example.com/cover.jpg' })} />)
    expect(screen.getByText(/Cover \+/)).toBeInTheDocument()
  })

  it('does not show Cover + when cover URL is empty', () => {
    render(<DuplicateBookSummary ub={makeUB({ coverUrl: '' })} />)
    expect(screen.queryByText(/Cover \+/)).not.toBeInTheDocument()
  })

  it('shows Desc + when description is set', () => {
    render(<DuplicateBookSummary ub={makeUB({ description: 'A great book.' })} />)
    expect(screen.getByText(/Desc \+/)).toBeInTheDocument()
  })

  it('shows Open Library ID from externalRefs', () => {
    render(<DuplicateBookSummary ub={makeUB({ externalRefs: { openlibrary: 'OL12345M' } })} />)
    expect(screen.getByText(/OL OL12345M/)).toBeInTheDocument()
  })

  it('does not show OL indicator when externalRefs is empty', () => {
    render(<DuplicateBookSummary ub={makeUB({ externalRefs: {} })} />)
    expect(screen.queryByText(/OL /)).not.toBeInTheDocument()
  })

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

  it('renders all metadata fields together', () => {
    render(
      <DuplicateBookSummary
        ub={makeUB({
          isbn13: '9780261102217',
          isbn10: '0261102214',
          pageCount: 310,
          coverUrl: 'https://example.com/c.jpg',
          description: 'Epic fantasy.',
          externalRefs: { openlibrary: 'OL27448W' },
          formats: ['epub', 'kepub'],
          tags: ['own-physical'],
          status: 'read'
        })}
      />
    )
    expect(screen.getByText(/9780261102217/)).toBeInTheDocument()
    expect(screen.getByText(/310p/)).toBeInTheDocument()
    expect(screen.getByText(/Cover \+/)).toBeInTheDocument()
    expect(screen.getByText(/Desc \+/)).toBeInTheDocument()
    expect(screen.getByText(/OL OL27448W/)).toBeInTheDocument()
    expect(screen.getByText('EPUB')).toBeInTheDocument()
    expect(screen.getByText('KEPUB')).toBeInTheDocument()
    expect(screen.getByText('Physical')).toBeInTheDocument()
    expect(screen.getByText('read')).toBeInTheDocument()
  })
})
