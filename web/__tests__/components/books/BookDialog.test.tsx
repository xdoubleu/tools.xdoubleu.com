import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookDialog from '@/components/books/BookDialog'
import { useLibrary } from '@/hooks/useBooks'
import {
  ExternalBookResultSchema,
  GetLibraryResponseSchema,
  LibraryResponseSchema,
  BookShelfSchema
} from '@/lib/gen/books/v1/library_pb'

const mockAddBook = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useCreateBook: () => mockAddBook,
  useLibrary: jest.fn()
}))

const fakeBook = create(ExternalBookResultSchema, {
  provider: 'hardcover',
  providerId: '9780134190440',
  title: 'The Go Programming Language',
  authors: ['Alan Donovan', 'Brian Kernighan'],
  isbn13: '9780134190440',
  coverUrl: 'https://covers.example.com/go.jpg',
  description: 'A great book about Go.'
})

function makeLibraryData(shelfNames: string[] = []) {
  return create(GetLibraryResponseSchema, {
    library: create(LibraryResponseSchema, {
      shelves: shelfNames.map((name) => create(BookShelfSchema, { name }))
    })
  })
}

describe('BookDialog', () => {
  beforeEach(() => {
    mockAddBook.mockReset()
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData(),
      isLoading: false,
      error: undefined
    })
  })

  it('renders nothing when book is null', () => {
    const { container } = render(<BookDialog book={null} onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders book title and authors', () => {
    render(<BookDialog book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getByText('The Go Programming Language')).toBeInTheDocument()
    expect(screen.getByText('Alan Donovan, Brian Kernighan')).toBeInTheDocument()
  })

  it('renders status select defaulting to "Want to read"', () => {
    render(<BookDialog book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)
    const select = screen.getByLabelText('Status') as HTMLSelectElement
    expect(select.value).toBe('to-read')
  })

  it('includes custom shelves from the library as selectable options', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData(['book-club']),
      isLoading: false,
      error: undefined
    })
    render(<BookDialog book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)
    const select = screen.getByLabelText('Status') as HTMLSelectElement
    expect(screen.getByRole('option', { name: 'book-club' })).toBeInTheDocument()
    fireEvent.change(select, { target: { value: 'book-club' } })
    expect(select.value).toBe('book-club')
  })

  it('excludes built-in statuses from the custom shelf options', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData(['read', 'book-club']),
      isLoading: false,
      error: undefined
    })
    render(<BookDialog book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getAllByRole('option', { name: 'Read' })).toHaveLength(1)
  })

  it('calls onClose when Cancel button clicked', () => {
    const onClose = jest.fn()
    render(<BookDialog book={fakeBook} onClose={onClose} onAdded={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls addBook and onAdded on successful submit', async () => {
    const onAdded = jest.fn()
    const onClose = jest.fn()
    mockAddBook.mockResolvedValue(undefined)
    render(<BookDialog book={fakeBook} onClose={onClose} onAdded={onAdded} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))

    await waitFor(() => {
      expect(mockAddBook).toHaveBeenCalled()
      expect(onAdded).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('shows error message when addBook throws', async () => {
    mockAddBook.mockRejectedValue(new Error('Network error'))
    render(<BookDialog book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('closes when Escape is pressed', () => {
    const onClose = jest.fn()
    render(<BookDialog book={fakeBook} onClose={onClose} onAdded={jest.fn()} />)
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })
})
