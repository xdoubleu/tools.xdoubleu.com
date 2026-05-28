import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookModal from '@/components/backlog/BookModal'
import { ExternalBookResultSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockAddBook = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useAddBook: () => mockAddBook
}))

const fakeBook = create(ExternalBookResultSchema, {
  provider: 'hardcover',
  providerId: 'hc-123',
  title: 'The Go Programming Language',
  authors: ['Alan Donovan', 'Brian Kernighan'],
  isbn13: '9780134190440',
  coverUrl: 'https://covers.example.com/go.jpg',
  description: 'A great book about Go.'
})

describe('BookModal', () => {
  beforeEach(() => {
    mockAddBook.mockReset()
  })

  it('renders nothing when book is null', () => {
    const { container } = render(<BookModal book={null} onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders book title and authors', () => {
    render(<BookModal book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getByText('The Go Programming Language')).toBeInTheDocument()
    expect(screen.getByText('Alan Donovan, Brian Kernighan')).toBeInTheDocument()
  })

  it('renders status select with wishlist default', () => {
    render(<BookModal book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)
    const select = screen.getByLabelText('Status') as HTMLSelectElement
    expect(select.value).toBe('wishlist')
  })

  it('calls onClose when Cancel button clicked', () => {
    const onClose = jest.fn()
    render(<BookModal book={fakeBook} onClose={onClose} onAdded={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls addBook and onAdded on successful submit', async () => {
    const onAdded = jest.fn()
    const onClose = jest.fn()
    mockAddBook.mockResolvedValue(undefined)
    render(<BookModal book={fakeBook} onClose={onClose} onAdded={onAdded} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))

    await waitFor(() => {
      expect(mockAddBook).toHaveBeenCalled()
      expect(onAdded).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('shows error message when addBook throws', async () => {
    mockAddBook.mockRejectedValue(new Error('Network error'))
    render(<BookModal book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('renders notes textarea', () => {
    render(<BookModal book={fakeBook} onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getByLabelText('Notes')).toBeInTheDocument()
  })

  it('closes when clicking the backdrop', () => {
    const onClose = jest.fn()
    const { container } = render(
      <BookModal book={fakeBook} onClose={onClose} onAdded={jest.fn()} />
    )
    // The outer backdrop div is the first child
    fireEvent.click(container.querySelector('.fixed')!)
    expect(onClose).toHaveBeenCalled()
  })
})
