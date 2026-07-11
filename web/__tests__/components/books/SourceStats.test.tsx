import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import SourceStats from '@/components/books/SourceStats'

const mockStatsData: {
  data:
    | {
        sources: { source: string; foundCount: number; uniqueCount: number }[]
        totalBooks: number
        notFoundAnywhere: number
        neverScanned: number
      }
    | undefined
  isLoading: boolean
  error: Error | undefined
} = {
  data: undefined,
  isLoading: false,
  error: undefined
}

const mockUniqueBooksData: {
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
  useSourceUniqueBooks: () => mockUniqueBooksData
}))

beforeEach(() => {
  mockStatsData.data = undefined
  mockStatsData.isLoading = false
  mockStatsData.error = undefined
  mockUniqueBooksData.data = undefined
  mockUniqueBooksData.isLoading = false
  mockUniqueBooksData.error = undefined
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

  it('renders one row per source with found/unique counts and the totals', () => {
    mockStatsData.data = {
      sources: [
        { source: 'openlibrary', foundCount: 42, uniqueCount: 7 },
        { source: 'googlebooks', foundCount: 30, uniqueCount: 2 },
        { source: 'unicat', foundCount: 5, uniqueCount: 1 }
      ],
      totalBooks: 50,
      notFoundAnywhere: 4,
      neverScanned: 3
    }
    render(<SourceStats />)

    expect(screen.getByText('Open Library')).toBeInTheDocument()
    expect(screen.getByText('Google Books')).toBeInTheDocument()
    expect(screen.getByText('UniCat')).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByText('7')).toBeInTheDocument()
    expect(screen.getByText('50 books in the catalog.')).toBeInTheDocument()
    expect(screen.getByText('4 scanned but not found in any source.')).toBeInTheDocument()
    expect(screen.getByText('3 never scanned.')).toBeInTheDocument()
  })

  it('opens a dialog listing the unique books when a Unique count is clicked', () => {
    mockStatsData.data = {
      sources: [{ source: 'unicat', foundCount: 5, uniqueCount: 1 }],
      totalBooks: 50,
      notFoundAnywhere: 4,
      neverScanned: 3
    }
    mockUniqueBooksData.data = {
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
      sources: [{ source: 'unicat', foundCount: 5, uniqueCount: 0 }],
      totalBooks: 50,
      notFoundAnywhere: 4,
      neverScanned: 3
    }
    render(<SourceStats />)

    expect(screen.queryByRole('button', { name: '0' })).not.toBeInTheDocument()
  })
})
