import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import BookSearchBar from '@/components/backlog/BookSearchBar'

const mockSearchLibrary = jest.fn()
const mockSearchExternal = jest.fn()
const mockAddBook = jest.fn()
const mockRouterPush = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useSearchLibrary: () => mockSearchLibrary,
  useSearchExternal: () => mockSearchExternal,
  useAddBook: () => mockAddBook
}))

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockRouterPush })
}))

jest.useFakeTimers()

const EXTERNAL_BOOK = {
  provider: 'openlibrary',
  providerId: '1',
  title: 'Go Book',
  authors: ['Donovan'],
  isbn13: '',
  coverUrl: '',
  description: ''
}

const LIBRARY_USER_BOOK = {
  id: 'ub-1',
  userId: 'user-1',
  bookId: 'b-1',
  book: {
    id: 'b-1',
    title: 'My Owned Book',
    authors: ['Author A'],
    isbn13: '',
    coverUrl: '',
    description: '',
    pageCount: 0
  },
  status: 'reading',
  tags: [],
  rating: 0,
  notes: '',
  finishedAt: [],
  addedAt: '',
  updatedAt: '',
  progressMode: '',
  currentPage: 0,
  progressPercent: 0,
  formats: []
}

describe('BookSearchBar', () => {
  const onAdded = jest.fn()

  beforeEach(() => {
    mockSearchLibrary.mockReset()
    mockSearchExternal.mockReset()
    mockAddBook.mockReset()
    mockRouterPush.mockReset()
    onAdded.mockReset()
    // Default: library empty so external fallback fires in most tests
    mockSearchLibrary.mockResolvedValue({ books: [] })
  })

  afterEach(() => {
    jest.clearAllTimers()
  })

  it('renders search input', () => {
    render(<BookSearchBar onAdded={onAdded} />)
    expect(screen.getByPlaceholderText('Search books...')).toBeInTheDocument()
  })

  it('clears results when query is empty', async () => {
    render(<BookSearchBar onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books...')
    fireEvent.change(input, { target: { value: 'go' } })
    fireEvent.change(input, { target: { value: '' } })
    expect(mockSearchLibrary).not.toHaveBeenCalled()
    expect(mockSearchExternal).not.toHaveBeenCalled()
  })

  it('falls back to searchExternal when library is empty and shows results', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [] })
    mockSearchExternal.mockResolvedValue({ results: [EXTERNAL_BOOK] })

    render(<BookSearchBar onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'Go' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => {
      expect(mockSearchLibrary).toHaveBeenCalledWith('Go')
      expect(mockSearchExternal).toHaveBeenCalledWith('Go')
      expect(screen.getByText('Go Book')).toBeInTheDocument()
    })
  })

  it('shows library result and does not call searchExternal when library has matches', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [LIBRARY_USER_BOOK] })

    render(<BookSearchBar onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'My' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => {
      expect(mockSearchLibrary).toHaveBeenCalledWith('My')
      expect(mockSearchExternal).not.toHaveBeenCalled()
      expect(screen.getByText('My Owned Book')).toBeInTheDocument()
    })
  })

  it('navigates to book detail page and clears results when a library result is clicked', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [LIBRARY_USER_BOOK] })

    render(<BookSearchBar onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'My' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => screen.getByText('My Owned Book'))
    fireEvent.click(screen.getByText('My Owned Book'))

    expect(mockRouterPush).toHaveBeenCalledWith('/backlog/books/ub-1')
    // Add modal should NOT open
    expect(screen.queryByRole('button', { name: 'Add Book' })).not.toBeInTheDocument()
  })

  it('clears results when search fails', async () => {
    mockSearchLibrary.mockRejectedValue(new Error('Network error'))

    render(<BookSearchBar onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'fail' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => {
      expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    })
  })

  it('opens BookModal when an external search result is selected', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [] })
    mockSearchExternal.mockResolvedValue({
      results: [{ ...EXTERNAL_BOOK, authors: [] }]
    })

    render(<BookSearchBar onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books...')

    fireEvent.change(input, { target: { value: 'Go' } })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })

    await waitFor(() => screen.getByText('Go Book'))
    fireEvent.click(screen.getByText('Go Book'))

    expect(screen.getByRole('button', { name: 'Add Book' })).toBeInTheDocument()
  })
})
