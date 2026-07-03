import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import CompareReport from '@/components/books/CompareReport'
import {
  BookMismatchSchema,
  BookRefSchema,
  CompareCSVResponseSchema
} from '@/lib/gen/books/v1/catalog_pb'
import type { BookMismatch, CompareCSVResponse } from '@/lib/gen/books/v1/catalog_pb'

function makeResult(overrides = {}): CompareCSVResponse {
  return create(CompareCSVResponseSchema, overrides)
}

function makeMismatch(
  differences: string[],
  csvTitle = 'CSV Book',
  libTitle = 'Lib Book'
): BookMismatch {
  return create(BookMismatchSchema, {
    differences,
    csv: differences.includes('missing-in-csv')
      ? undefined
      : create(BookRefSchema, {
          title: csvTitle,
          authors: ['Author A'],
          isbn13: '9780000000001',
          status: 'read'
        }),
    library: differences.includes('missing-in-library')
      ? undefined
      : create(BookRefSchema, {
          title: libTitle,
          authors: ['Author A'],
          isbn13: '9780000000001',
          status: 'to-read'
        })
  })
}

describe('CompareReport', () => {
  it('shows all-match message when no mismatches', () => {
    render(<CompareReport result={makeResult({ csvCount: 5, libraryCount: 5, matchedCount: 5 })} />)
    expect(screen.getByText(/CSV matches library exactly/i)).toBeInTheDocument()
  })

  it('renders summary counts', () => {
    const result = makeResult({
      csvCount: 10,
      libraryCount: 8,
      matchedCount: 6,
      mismatches: [makeMismatch(['missing-in-library'])]
    })
    render(<CompareReport result={result} />)
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('8')).toBeInTheDocument()
    expect(screen.getByText('6')).toBeInTheDocument()
  })

  it('shows missing-in-library group with CSV title', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['missing-in-library'], 'Gone Book')]
    })
    render(<CompareReport result={result} />)
    expect(screen.getByText(/Only in CSV/i)).toBeInTheDocument()
    expect(screen.getByText('Gone Book — Author A')).toBeInTheDocument()
  })

  it('shows missing-in-csv group with library title', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['missing-in-csv'], '', 'Orphan Book')]
    })
    render(<CompareReport result={result} />)
    expect(screen.getByText(/Only in library/i)).toBeInTheDocument()
    expect(screen.getByText('Orphan Book — Author A')).toBeInTheDocument()
  })

  it('shows status diff group', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['status'])]
    })
    render(<CompareReport result={result} />)
    expect(screen.getByText(/Reading state differs/i)).toBeInTheDocument()
  })

  it('shows isbn diff group', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['isbn'])]
    })
    render(<CompareReport result={result} />)
    expect(screen.getByText(/ISBN differs/i)).toBeInTheDocument()
  })

  it('shows title diff group', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['title'])]
    })
    render(<CompareReport result={result} />)
    expect(screen.getByText(/Title differs/i)).toBeInTheDocument()
  })

  it('does not render empty groups', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['missing-in-library'])]
    })
    render(<CompareReport result={result} />)
    // Only the CSV-only group has items; library-only and others should not render
    expect(screen.queryByText(/Only in library/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Reading state differs/i)).not.toBeInTheDocument()
  })
})
