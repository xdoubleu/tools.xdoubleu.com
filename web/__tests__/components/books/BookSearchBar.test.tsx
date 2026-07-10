import React from 'react'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import BookSearchBar from '@/components/books/BookSearchBar'

const mockSearchLibrary = jest.fn()
const mockSearchExternal = jest.fn()
const mockAddBook = jest.fn()
const mockRouterPush = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useSearchLibrary: () => mockSearchLibrary,
  useSearchExternal: () => mockSearchExternal,
  useCreateBook: () => mockAddBook
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
    expect(screen.getByPlaceholderText('Search books…')).toBeInTheDocument()
  })

  it('shows library results and navigates on click', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [LIBRARY_USER_BOOK] })
    render(<BookSearchBar onAdded={onAdded} />)
    fireEvent.change(screen.getByPlaceholderText('Search books…'), {
      target: { value: 'My' }
    })
    await act(async () => {
      jest.advanceTimersByTime(300)
    })
    await waitFor(() => screen.getByText('My Owned Book'))
    fireEvent.click(screen.getByText('My Owned Book'))
    expect(mockRouterPush).toHaveBeenCalledWith('/books/ub-1')
  })

  it('falls back to OL when library has no results', async () => {
    mockSearchLibrary.mockResolvedValue({ books: [] })
    mockSearchExternal.mockResolvedValue({ results: [EXTERNAL_BOOK] })
    render(<BookSearchBar onAdded={onAdded} />)
    fireEvent.change(screen.getByPlaceholderText('Search books…'), {
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
    fireEvent.change(screen.getByPlaceholderText('Search books…'), {
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
    const input = screen.getByPlaceholderText('Search books…')
    fireEvent.change(input, { target: { value: 'go' } })
    fireEvent.change(input, { target: { value: '' } })
    expect(mockSearchLibrary).not.toHaveBeenCalled()
    expect(mockSearchExternal).not.toHaveBeenCalled()
  })
})

// ---------------------------------------------------------------------------
// Controlled mode (library page — query/onChange provided). No dropdown, no
// OL fallback here — BooksLibrary owns result rendering (see its own tests).
// ---------------------------------------------------------------------------
describe('BookSearchBar — controlled mode', () => {
  const onAdded = jest.fn()

  beforeEach(() => {
    mockSearchLibrary.mockReset()
    mockSearchExternal.mockReset()
    mockAddBook.mockReset()
    onAdded.mockReset()
  })

  it('renders search input with controlled value', () => {
    render(<BookSearchBar query="dune" onChange={jest.fn()} onAdded={onAdded} />)
    expect(screen.getByDisplayValue('dune')).toBeInTheDocument()
  })

  it('updates the input immediately but debounces onChange', () => {
    const onChange = jest.fn()
    render(<BookSearchBar query="" onChange={onChange} onAdded={onAdded} />)
    const input = screen.getByPlaceholderText('Search books…')
    fireEvent.change(input, { target: { value: 'dune' } })
    expect(input).toHaveValue('dune')
    expect(onChange).not.toHaveBeenCalled()
    act(() => {
      jest.advanceTimersByTime(300)
    })
    expect(onChange).toHaveBeenCalledWith('dune')
  })

  it('syncs the input from the query prop (browser back/forward)', () => {
    const { rerender } = render(
      <BookSearchBar query="dune" onChange={jest.fn()} onAdded={onAdded} />
    )
    expect(screen.getByDisplayValue('dune')).toBeInTheDocument()
    rerender(<BookSearchBar query="foundation" onChange={jest.fn()} onAdded={onAdded} />)
    expect(screen.getByDisplayValue('foundation')).toBeInTheDocument()
  })

  it('never searches the library or Open Library directly', () => {
    render(<BookSearchBar query="Go" onChange={jest.fn()} onAdded={onAdded} />)
    expect(mockSearchLibrary).not.toHaveBeenCalled()
    expect(mockSearchExternal).not.toHaveBeenCalled()
  })
})
