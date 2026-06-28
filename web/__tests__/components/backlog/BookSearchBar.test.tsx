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
  book: { title: 'My Owned Book', authors: ['Author A'] }
}

// ---------------------------------------------------------------------------
// Standalone mode (dashboard — no query/onChange/hasLibraryResults props)
// ---------------------------------------------------------------------------
describe('BookSearchBar — standalone mode', () => {
  const onAdded = jest.fn()

  beforeEach(() => {
    mockSearchLibrary.mockReset()
    mockSearchExternal.mockReset()
    mockAddBook.mockReset()
    mockRouterPush.mockReset()
    onAdded.mockReset()
  })

  afterEach(() => {
    jest.clearAllTimers()
  })

  it('renders search input', () => {
    render(<BookSearchBar onAdded={onAdded} />)
    expect(screen.getByPlaceholderText('Search books...')).toBeInTheDocument()
  })

  it('shows library results and navigates on click', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [LIBRARY_USER_BOOK] })
    render(<BookSearchBar onAdded={onAdded} />)
    fireEvent.change(screen.getByPlaceholderText('Search books...'), {
      target: { value: 'My' }
    })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('My Owned Book'))
    fireEvent.click(screen.getByText('My Owned Book'))
    expect(mockRouterPush).toHaveBeenCalledWith('/backlog/books/ub-1')
  })

  it('falls back to OL when library has no results', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [] })
    mockSearchExternal.mockResolvedValue({ results: [EXTERNAL_BOOK] })
    render(<BookSearchBar onAdded={onAdded} />)
    fireEvent.change(screen.getByPlaceholderText('Search books...'), {
      target: { value: 'Go' }
    })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => {
      expect(mockSearchExternal).toHaveBeenCalledWith('Go')
      expect(screen.getByText('Go Book')).toBeInTheDocument()
    })
  })

  it('opens BookModal when an OL result is clicked', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [] })
    mockSearchExternal.mockResolvedValue({ results: [{ ...EXTERNAL_BOOK, authors: [] }] })
    render(<BookSearchBar onAdded={onAdded} />)
    fireEvent.change(screen.getByPlaceholderText('Search books...'), {
      target: { value: 'Go' }
    })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('Go Book'))
    fireEvent.click(screen.getByText('Go Book'))
    expect(screen.getByRole('button', { name: 'Add Book' })).toBeInTheDocument()
  })

  it('clears results when query is emptied', () => {
    render(<BookSearchBar onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books...')
    fireEvent.change(input, { target: { value: 'go' } })
    fireEvent.change(input, { target: { value: '' } })
    expect(mockSearchLibrary).not.toHaveBeenCalled()
    expect(mockSearchExternal).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// Controlled mode (library page — query/onChange/hasLibraryResults provided)
// ---------------------------------------------------------------------------
describe('BookSearchBar — controlled mode', () => {
  const onAdded = jest.fn()

  beforeEach(() => {
    mockSearchLibrary.mockReset()
    mockSearchExternal.mockReset()
    mockAddBook.mockReset()
    onAdded.mockReset()
  })

  afterEach(() => {
    jest.clearAllTimers()
  })

  it('renders search input with controlled value', () => {
    render(
      <BookSearchBar query="dune" onChange={jest.fn()} onAdded={onAdded} hasLibraryResults={true} />
    )
    expect(screen.getByDisplayValue('dune')).toBeInTheDocument()
  })

  it('does not call searchExternal when library has results', async () => {
    mockSearchExternal.mockResolvedValue({ results: [EXTERNAL_BOOK] })
    render(
      <BookSearchBar query="Go" onChange={jest.fn()} onAdded={onAdded} hasLibraryResults={true} />
    )
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    expect(mockSearchExternal).not.toHaveBeenCalled()
  })

  it('calls searchExternal and shows dropdown when library has no results', async () => {
    mockSearchExternal.mockResolvedValue({ results: [EXTERNAL_BOOK] })
    render(
      <BookSearchBar query="Go" onChange={jest.fn()} onAdded={onAdded} hasLibraryResults={false} />
    )
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => {
      expect(mockSearchExternal).toHaveBeenCalledWith('Go')
      expect(screen.getByText('Go Book')).toBeInTheDocument()
    })
  })

  it('hides dropdown when hasLibraryResults flips back to true', async () => {
    mockSearchExternal.mockResolvedValue({ results: [EXTERNAL_BOOK] })
    const { rerender } = render(
      <BookSearchBar query="Go" onChange={jest.fn()} onAdded={onAdded} hasLibraryResults={false} />
    )
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('Go Book'))

    rerender(
      <BookSearchBar query="Go" onChange={jest.fn()} onAdded={onAdded} hasLibraryResults={true} />
    )
    await waitFor(() => {
      expect(screen.queryByText('Go Book')).not.toBeInTheDocument()
    })
  })

  it('calls onChange when user types', () => {
    const onChange = jest.fn()
    render(
      <BookSearchBar query="" onChange={onChange} onAdded={onAdded} hasLibraryResults={true} />
    )
    fireEvent.change(screen.getByPlaceholderText('Search books...'), {
      target: { value: 'dune' }
    })
    expect(onChange).toHaveBeenCalledWith('dune')
  })

  it('opens BookModal when an OL result is clicked', async () => {
    mockSearchExternal.mockResolvedValue({ results: [{ ...EXTERNAL_BOOK, authors: [] }] })
    render(
      <BookSearchBar query="Go" onChange={jest.fn()} onAdded={onAdded} hasLibraryResults={false} />
    )
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('Go Book'))
    fireEvent.click(screen.getByText('Go Book'))
    expect(screen.getByRole('button', { name: 'Add Book' })).toBeInTheDocument()
  })

  it('shows searching indicator while OL search is in flight', async () => {
    mockSearchExternal.mockReturnValue(new Promise(() => {}))
    render(
      <BookSearchBar query="Go" onChange={jest.fn()} onAdded={onAdded} hasLibraryResults={false} />
    )
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    expect(screen.getByText('Searching...')).toBeInTheDocument()
  })

  it('clears results when OL search fails', async () => {
    mockSearchExternal.mockRejectedValue(new Error('Network error'))
    render(
      <BookSearchBar
        query="fail"
        onChange={jest.fn()}
        onAdded={onAdded}
        hasLibraryResults={false}
      />
    )
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => {
      expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    })
  })
})
