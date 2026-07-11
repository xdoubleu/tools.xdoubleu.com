import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookSourceSync from '@/components/books/BookSourceSync'

const mockApplyBookSource = jest.fn()
const mockSourcesData: {
  data: { proposal: unknown } | undefined
  isLoading: boolean
  error: Error | undefined
} = {
  data: undefined,
  isLoading: false,
  error: undefined
}
const mockMutate = jest.fn()

const mockUseBookSources: jest.Mock<typeof mockSourcesData, unknown[]> = jest.fn(
  () => mockSourcesData
)

jest.mock('@/hooks/useBooks', () => ({
  useBookSources: (...args: unknown[]) => mockUseBookSources(...args),
  useApplyBookSource: () => mockApplyBookSource
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

beforeEach(() => {
  mockApplyBookSource.mockReset().mockResolvedValue({})
  mockUseBookSources.mockClear()
  mockMutate.mockReset()
  mockSourcesData.data = undefined
  mockSourcesData.isLoading = false
  mockSourcesData.error = undefined
})

describe('BookSourceSync', () => {
  it('shows a button and does not fetch until clicked', () => {
    render(<BookSourceSync bookId="b1" />)
    expect(screen.getByRole('button', { name: /sync metadata source/i })).toBeInTheDocument()
    expect(screen.queryByText(/fetching sources/i)).not.toBeInTheDocument()
  })

  it('shows a loading state after the button is clicked', () => {
    mockSourcesData.isLoading = true
    render(<BookSourceSync bookId="b1" />)
    fireEvent.click(screen.getByRole('button', { name: /sync metadata source/i }))
    expect(screen.getByText(/fetching sources/i)).toBeInTheDocument()
  })

  it('shows an error state when the fetch fails', () => {
    mockSourcesData.error = new Error('boom')
    render(<BookSourceSync bookId="b1" />)
    fireEvent.click(screen.getByRole('button', { name: /sync metadata source/i }))
    expect(screen.getByText(/failed to fetch sources/i)).toBeInTheDocument()
  })

  it('renders the fetched proposal and applies the chosen source', async () => {
    mockSourcesData.data = {
      proposal: {
        bookId: 'b1',
        library: {
          source: '',
          title: 'Dune',
          authors: [],
          coverUrl: '',
          description: '',
          pageCount: 0,
          isbn13: '',
          differs: []
        },
        sources: [
          {
            source: 'openlibrary',
            title: 'Dune (OL)',
            authors: ['Frank Herbert'],
            coverUrl: '',
            description: '',
            pageCount: 0,
            isbn13: '',
            differs: ['authors']
          }
        ]
      }
    }
    render(<BookSourceSync bookId="b1" />)
    fireEvent.click(screen.getByRole('button', { name: /sync metadata source/i }))

    fireEvent.click(screen.getByRole('radio', { name: 'Open Library' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() =>
      expect(mockApplyBookSource).toHaveBeenCalledWith('b1', 'openlibrary', undefined)
    )
    expect(mockMutate).toHaveBeenCalledWith('/books')
  })

  it('re-fetches with tweaked search terms and applies with the override', async () => {
    mockSourcesData.data = {
      proposal: {
        bookId: 'b1',
        library: {
          source: '',
          title: 'Misspelled Titel',
          authors: ['Author'],
          coverUrl: '',
          description: '',
          pageCount: 0,
          isbn13: '',
          differs: []
        },
        sources: []
      }
    }
    render(<BookSourceSync bookId="b1" />)
    fireEvent.click(screen.getByRole('button', { name: /sync metadata source/i }))

    expect(mockUseBookSources).toHaveBeenLastCalledWith('b1', true, undefined)

    fireEvent.change(screen.getByPlaceholderText('Title'), {
      target: { value: 'Correct Title' }
    })
    fireEvent.click(screen.getByRole('button', { name: /search with these terms/i }))

    const override = { title: 'Correct Title', author: 'Author' }
    expect(mockUseBookSources).toHaveBeenLastCalledWith('b1', true, override)

    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    await waitFor(() => expect(mockApplyBookSource).toHaveBeenCalledWith('b1', '', override))
  })
})
