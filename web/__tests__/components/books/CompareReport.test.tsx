import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  BookMismatchSchema,
  BookRefSchema,
  CompareCSVResponseSchema
} from '@/lib/gen/books/v1/catalog_pb'
import type { BookMismatch, CompareCSVResponse } from '@/lib/gen/books/v1/catalog_pb'

const mockApplyCSVFix = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBooks', () => ({
  useApplyCSVFix: () => mockApplyCSVFix
}))

import CompareReport from '@/components/books/CompareReport'

function makeResult(overrides = {}): CompareCSVResponse {
  return create(CompareCSVResponseSchema, overrides)
}

function makeMismatch(
  differences: string[],
  csvTitle = 'CSV Book',
  libTitle = 'Lib Book',
  id = 'mismatch-1',
  tags?: { csv: string[]; library: string[] },
  status: { csv: string; library: string } = { csv: 'read', library: 'to-read' }
): BookMismatch {
  return create(BookMismatchSchema, {
    id,
    differences,
    csv: differences.includes('missing-in-csv')
      ? undefined
      : create(BookRefSchema, {
          title: csvTitle,
          authors: ['Author A'],
          isbn13: '9780000000001',
          status: status.csv,
          tags: tags?.csv ?? []
        }),
    library: differences.includes('missing-in-library')
      ? undefined
      : create(BookRefSchema, {
          title: libTitle,
          authors: ['Author A'],
          isbn13: '9780000000001',
          status: status.library,
          tags: tags?.library ?? []
        })
  })
}

const onFixed = jest.fn()

describe('CompareReport', () => {
  beforeEach(() => {
    mockApplyCSVFix.mockReset()
    mockMutate.mockReset()
    onFixed.mockReset()
    mockApplyCSVFix.mockResolvedValue({})
  })

  it('shows all-match message when no mismatches', () => {
    render(
      <CompareReport
        result={makeResult({ csvCount: 5, libraryCount: 5, matchedCount: 5 })}
        csvData="csv"
        onFixed={onFixed}
      />
    )
    expect(screen.getByText(/CSV matches library exactly/i)).toBeInTheDocument()
  })

  it('renders summary counts', () => {
    const result = makeResult({
      csvCount: 10,
      libraryCount: 8,
      matchedCount: 6,
      mismatches: [makeMismatch(['missing-in-library'])]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('8')).toBeInTheDocument()
    expect(screen.getByText('6')).toBeInTheDocument()
  })

  it('shows missing-in-library group with CSV title', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['missing-in-library'], 'Gone Book')]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    expect(screen.getByText(/Only in CSV/i)).toBeInTheDocument()
    expect(screen.getByText('Gone Book — Author A')).toBeInTheDocument()
  })

  it('shows missing-in-csv group with library title and no fix button', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['missing-in-csv'], '', 'Orphan Book')]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    expect(screen.getByText(/Only in library/i)).toBeInTheDocument()
    expect(screen.getByText('Orphan Book — Author A')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^fix$/i })).not.toBeInTheDocument()
  })

  it('shows status diff group', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['status'])]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    expect(screen.getByText(/Reading state differs/i)).toBeInTheDocument()
  })

  it('shows isbn diff group', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['isbn'])]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    expect(screen.getByText(/ISBN differs/i)).toBeInTheDocument()
  })

  it('shows title diff group', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['title'])]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    expect(screen.getByText(/Title differs/i)).toBeInTheDocument()
  })

  it('shows tags diff group with added/removed badges', () => {
    const result = makeResult({
      mismatches: [
        makeMismatch(['tags'], 'CSV Book', 'Lib Book', 'mismatch-1', {
          csv: ['technical'],
          library: ['technical', 'wishlist']
        })
      ]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    expect(screen.getByText(/Tags differ/i)).toBeInTheDocument()
    expect(screen.getByText('−wishlist')).toBeInTheDocument()
  })

  it('shows before → after for a status fix', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['status'])]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    // Scoped to the row: the shelf-filter <select> also renders "read"/"to-read"
    // as option text, which would collide with a page-wide getByText.
    const row = screen.getByRole('listitem')
    expect(row).toHaveTextContent('to-read')
    expect(row).toHaveTextContent('read')
  })

  it('filters mismatches by shelf', () => {
    const result = makeResult({
      mismatches: [
        makeMismatch(['title'], 'Read Shelf Book', 'Read Shelf Book', 'm-read', undefined, {
          csv: 'read',
          library: 'read'
        }),
        makeMismatch(['isbn'], 'Other Book', 'Other Book', 'm-other', undefined, {
          csv: 'currently-reading',
          library: 'currently-reading'
        })
      ]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)

    fireEvent.change(screen.getByLabelText(/filter by shelf/i), {
      target: { value: 'read' }
    })

    expect(screen.getByText(/Title differs/i)).toBeInTheDocument()
    expect(screen.queryByText(/ISBN differs/i)).not.toBeInTheDocument()
  })

  it('does not render empty groups', () => {
    const result = makeResult({
      mismatches: [makeMismatch(['missing-in-library'])]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)
    // Only the CSV-only group has items; library-only and others should not render
    expect(screen.queryByText(/Only in library/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Reading state differs/i)).not.toBeInTheDocument()
  })

  it('applies a fix, revalidates the library cache, and re-runs the compare', async () => {
    const result = makeResult({
      mismatches: [makeMismatch(['status'], 'CSV Book', 'Lib Book', 'book-42')]
    })
    render(<CompareReport result={result} csvData="the-csv" onFixed={onFixed} />)

    fireEvent.click(screen.getByRole('button', { name: /^fix$/i }))

    await waitFor(() => expect(onFixed).toHaveBeenCalled())
    expect(mockApplyCSVFix).toHaveBeenCalledWith('the-csv', 'book-42', 'status')
    expect(mockMutate).toHaveBeenCalledWith('/books')
  })

  it('shows an error when the fix fails', async () => {
    mockApplyCSVFix.mockRejectedValue(new Error('boom'))
    const result = makeResult({
      mismatches: [makeMismatch(['isbn'])]
    })
    render(<CompareReport result={result} csvData="csv" onFixed={onFixed} />)

    fireEvent.click(screen.getByRole('button', { name: /^fix$/i }))

    expect(await screen.findByText(/fix failed/i)).toBeInTheDocument()
    expect(onFixed).not.toHaveBeenCalled()
  })
})
