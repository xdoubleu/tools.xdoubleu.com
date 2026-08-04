import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'
import BookRemoveAction from '@/components/books/BookRemoveAction'

const mockRemoveBook = jest.fn()
const mockUpdateBookStatus = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useRemoveBook: () => mockRemoveBook,
  useUpdateBookStatus: () => mockUpdateBookStatus
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

function makeBook(title: string) {
  return create(UserBookSchema, {
    id: '1',
    bookId: 'book-1',
    status: 'to-read',
    tags: [],
    formats: [],
    addedAt: '2024-01-01T00:00:00Z',
    book: create(BookSchema, { title, authors: ['Author'], pageCount: 300 })
  })
}

describe('BookRemoveAction', () => {
  beforeEach(() => {
    mockRemoveBook.mockReset()
    mockMutate.mockReset()
  })

  it('renders a remove button with an accessible label', () => {
    render(<BookRemoveAction userBook={makeBook('Dune')} />)
    expect(screen.getByRole('button', { name: 'Remove Dune from library' })).toBeInTheDocument()
  })

  it('opens the confirm dialog when clicked', () => {
    render(<BookRemoveAction userBook={makeBook('Dune')} />)
    fireEvent.click(screen.getByRole('button', { name: 'Remove Dune from library' }))
    expect(screen.getByText('Remove from library')).toBeInTheDocument()
  })

  it('calls removeBook and onSaved on confirm', async () => {
    mockRemoveBook.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    const onSaved = jest.fn()
    render(<BookRemoveAction userBook={makeBook('Dune')} onSaved={onSaved} />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove Dune from library' }))
    fireEvent.click(screen.getByTestId('remove-book-confirm-btn'))

    await waitFor(() => expect(mockRemoveBook).toHaveBeenCalledWith('book-1'))
    expect(onSaved).toHaveBeenCalled()
  })

  it('falls back to a generic label when the book is missing', () => {
    const userBook = create(UserBookSchema, {
      id: '1',
      bookId: 'book-1',
      status: 'to-read',
      tags: [],
      formats: [],
      addedAt: '2024-01-01T00:00:00Z'
    })
    render(<BookRemoveAction userBook={userBook} />)
    expect(screen.getByRole('button', { name: 'Remove book from library' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Remove book from library' }))
    expect(screen.getByText('this book')).toBeInTheDocument()
  })
})
