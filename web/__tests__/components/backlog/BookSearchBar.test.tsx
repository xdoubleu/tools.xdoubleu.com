import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import BookSearchBar from '@/components/backlog/BookSearchBar'

const mockSearchExternal = jest.fn()
const mockImportBooks = jest.fn()
const mockAddBook = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useSearchExternal: () => mockSearchExternal,
  useImportBooks: () => mockImportBooks,
  useAddBook: () => mockAddBook
}))

jest.useFakeTimers()

describe('BookSearchBar', () => {
  beforeEach(() => {
    mockSearchExternal.mockReset()
    mockImportBooks.mockReset()
    mockAddBook.mockReset()
  })

  afterEach(() => {
    jest.clearAllTimers()
  })

  it('renders search input', () => {
    render(<BookSearchBar onAdded={jest.fn()} />)
    expect(screen.getByPlaceholderText('Search books...')).toBeInTheDocument()
  })

  it('clears results when query is empty', async () => {
    render(<BookSearchBar onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText('Search books...')
    fireEvent.change(input, { target: { value: 'go' } })
    fireEvent.change(input, { target: { value: '' } })
    expect(mockSearchExternal).not.toHaveBeenCalled()
  })

  it('calls searchExternal after debounce and shows results', async () => {
    mockSearchExternal.mockResolvedValue({
      results: [
        {
          provider: 'hc',
          providerId: '1',
          title: 'Go Book',
          authors: ['Donovan'],
          isbn13: '',
          coverUrl: '',
          description: ''
        }
      ]
    })
    render(<BookSearchBar onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'Go' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => {
      expect(mockSearchExternal).toHaveBeenCalledWith('Go')
      expect(screen.getByText('Go Book')).toBeInTheDocument()
    })
  })

  it('clears results when search fails', async () => {
    mockSearchExternal.mockRejectedValue(new Error('Network error'))
    render(<BookSearchBar onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'fail' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => {
      expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    })
  })

  it('opens BookModal when a search result is selected', async () => {
    mockSearchExternal.mockResolvedValue({
      results: [
        {
          provider: 'hc',
          providerId: '1',
          title: 'Go Book',
          authors: [],
          isbn13: '',
          coverUrl: '',
          description: ''
        }
      ]
    })
    render(<BookSearchBar onAdded={jest.fn()} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'Go' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => screen.getByText('Go Book'))
    fireEvent.click(screen.getByText('Go Book'))

    // BookModal should be visible with the Add Book button
    expect(screen.getByRole('button', { name: 'Add Book' })).toBeInTheDocument()
  })
})
