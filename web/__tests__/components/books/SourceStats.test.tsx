import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import SourceStats from '@/components/books/SourceStats'

const mockStatsData: {
  data:
    | {
        sources: {
          source: string
          foundCount: number
          uniqueCount: number
          missedCount: number
        }[]
        totalBooks: number
        notFoundAnywhere: number
        neverScanned: number
        overlaps: { sources: string[]; count: number }[]
        missedOverlaps: { sources: string[]; count: number }[]
      }
    | undefined
  isLoading: boolean
  error: Error | undefined
} = {
  data: undefined,
  isLoading: false,
  error: undefined
}

const mockExactSourcesData: {
  data: { books: { id: string; title: string; authors: string[]; coverUrl: string }[] } | undefined
  isLoading: boolean
  error: Error | undefined
} = {
  data: undefined,
  isLoading: false,
  error: undefined
}

jest.mock('@/hooks/useBooks', () => ({
  useSourceStats: () => mockStatsData,
  useBooksInExactSources: () => mockExactSourcesData
}))

beforeEach(() => {
  mockStatsData.data = undefined
  mockStatsData.isLoading = false
  mockStatsData.error = undefined
  mockExactSourcesData.data = undefined
  mockExactSourcesData.isLoading = false
  mockExactSourcesData.error = undefined
})

describe('SourceStats', () => {
  it('shows a loading state', () => {
    mockStatsData.isLoading = true
    render(<SourceStats />)
    expect(screen.getByText(/loading/i)).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockStatsData.error = new Error('boom')
    render(<SourceStats />)
    expect(screen.getByText(/failed to load source stats/i)).toBeInTheDocument()
  })

  it('renders one row per source with found/missed/unique counts and the totals', () => {
    mockStatsData.data = {
      sources: [
        { source: 'hardcover', foundCount: 30, uniqueCount: 2, missedCount: 20 },
        { source: 'unicat', foundCount: 5, uniqueCount: 1, missedCount: 45 }
      ],
      totalBooks: 50,
      notFoundAnywhere: 4,
      neverScanned: 3,
      overlaps: [],
      missedOverlaps: []
    }
    render(<SourceStats />)

    expect(screen.getByText('Hardcover')).toBeInTheDocument()
    expect(screen.getByText('UniCat')).toBeInTheDocument()
    expect(screen.getByText('30')).toBeInTheDocument()
    expect(screen.getByText('20')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('43 found across all sources (in at least one).')).toBeInTheDocument()
    expect(screen.getByText('50 books in the catalog.')).toBeInTheDocument()
    expect(screen.getByText('4 missing from all sources.')).toBeInTheDocument()
    expect(screen.getByText('3 never scanned.')).toBeInTheDocument()
  })

  it('opens a dialog listing the unique books when a Unique count is clicked', () => {
    mockStatsData.data = {
      sources: [{ source: 'unicat', foundCount: 5, uniqueCount: 1, missedCount: 44 }],
      totalBooks: 50,
      notFoundAnywhere: 4,
      neverScanned: 3,
      overlaps: [],
      missedOverlaps: []
    }
    mockExactSourcesData.data = {
      books: [{ id: 'b1', title: 'De Kleine Bibliotheek', authors: ['Iemand'], coverUrl: '' }]
    }
    render(<SourceStats />)

    fireEvent.click(screen.getByText('1'))

    expect(screen.getByText('Unique to UniCat')).toBeInTheDocument()
    expect(screen.getByText('De Kleine Bibliotheek')).toBeInTheDocument()
    expect(screen.getByText('Iemand')).toBeInTheDocument()
  })

  it('does not render a click target when a source has zero unique books', () => {
    mockStatsData.data = {
      sources: [{ source: 'unicat', foundCount: 5, uniqueCount: 0, missedCount: 45 }],
      totalBooks: 50,
      notFoundAnywhere: 4,
      neverScanned: 3,
      overlaps: [],
      missedOverlaps: []
    }
    render(<SourceStats />)

    expect(screen.queryByRole('button', { name: '0' })).not.toBeInTheDocument()
  })

  // With only two configured sources (UniCat, Hardcover), the sole overlap
  // combo always spans every source — comboLabel renders it as "All sources"
  // rather than a specific pair.
  it('renders an overlap section and opens a dialog for the combo', () => {
    mockStatsData.data = {
      sources: [
        { source: 'unicat', foundCount: 5, uniqueCount: 1, missedCount: 45 },
        { source: 'hardcover', foundCount: 18, uniqueCount: 3, missedCount: 25 }
      ],
      totalBooks: 50,
      notFoundAnywhere: 4,
      neverScanned: 3,
      overlaps: [{ sources: ['unicat', 'hardcover'], count: 12 }],
      missedOverlaps: []
    }
    mockExactSourcesData.data = {
      books: [{ id: 'b2', title: 'Overlap Book', authors: [], coverUrl: '' }]
    }
    render(<SourceStats />)

    expect(screen.getByText('Overlap — found in exactly these sources')).toBeInTheDocument()
    expect(screen.getByText('All sources')).toBeInTheDocument()

    fireEvent.click(screen.getByText('12'))

    expect(screen.getByText('Found in All sources')).toBeInTheDocument()
    expect(screen.getByText('Overlap Book')).toBeInTheDocument()
  })

  it('does not render the overlap section when the combo is zero', () => {
    mockStatsData.data = {
      sources: [{ source: 'hardcover', foundCount: 5, uniqueCount: 5, missedCount: 0 }],
      totalBooks: 5,
      notFoundAnywhere: 0,
      neverScanned: 0,
      overlaps: [{ sources: ['unicat', 'hardcover'], count: 0 }],
      missedOverlaps: []
    }
    render(<SourceStats />)

    expect(screen.queryByText('Overlap — found in exactly these sources')).not.toBeInTheDocument()
  })

  it('renders a missed-overlaps section when the combo is nonzero', () => {
    mockStatsData.data = {
      sources: [{ source: 'hardcover', foundCount: 5, uniqueCount: 5, missedCount: 0 }],
      totalBooks: 5,
      notFoundAnywhere: 0,
      neverScanned: 0,
      overlaps: [],
      missedOverlaps: [{ sources: ['unicat', 'hardcover'], count: 3 }]
    }
    render(<SourceStats />)

    expect(
      screen.getByText('Missed overlaps — missed by exactly these sources')
    ).toBeInTheDocument()
    expect(screen.getByText('All sources')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('does not render the missed-overlaps section when the combo is zero', () => {
    mockStatsData.data = {
      sources: [{ source: 'hardcover', foundCount: 5, uniqueCount: 5, missedCount: 0 }],
      totalBooks: 5,
      notFoundAnywhere: 0,
      neverScanned: 0,
      overlaps: [],
      missedOverlaps: [{ sources: ['unicat', 'hardcover'], count: 0 }]
    }
    render(<SourceStats />)

    expect(
      screen.queryByText('Missed overlaps — missed by exactly these sources')
    ).not.toBeInTheDocument()
  })
})
