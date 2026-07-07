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
  id = 'mismatch-1'
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
