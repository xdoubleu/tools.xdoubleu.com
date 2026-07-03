import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

const mockUpdateBookStatus = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBooks', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus
}))

import BookFavouriteButton from '@/components/books/BookFavouriteButton'

function makeBook(tags: string[] = [], rating = 3, status = 'to-read') {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status,
    rating,
    tags,
    formats: [],
    book: create(BookSchema, { title: 'Test', authors: [] })
  })
}

describe('BookFavouriteButton', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockMutate.mockReset()
    mockUpdateBookStatus.mockResolvedValue({})
  })

  it('renders a button', () => {
    render(<BookFavouriteButton userBook={makeBook()} />)
    expect(screen.getByRole('button')).toBeInTheDocument()
  })

  it('is not pressed when not a favourite', () => {
    render(<BookFavouriteButton userBook={makeBook()} />)
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'false')
  })

  it('is pressed when already a favourite', () => {
    render(<BookFavouriteButton userBook={makeBook(['favourite'])} />)
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'true')
  })

  it('calls UpdateBookStatus with favourite=true when toggled on', async () => {
    render(<BookFavouriteButton userBook={makeBook(['own-physical'])} />)

    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith({
        bookId: 'book-1',
        status: 'to-read',
        favourite: true,
        rating: '3'
      })
    })
    expect(mockMutate).toHaveBeenCalledWith('/books')
  })

  it('calls UpdateBookStatus with favourite=false when toggled off', async () => {
    render(<BookFavouriteButton userBook={makeBook(['favourite'])} />)

    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({ favourite: false })
      )
    })
  })

  it('calls onSaved after successful save', async () => {
    const onSaved = jest.fn()
    render(<BookFavouriteButton userBook={makeBook()} onSaved={onSaved} />)

    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('reverts to previous state on error', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('fail'))
    render(<BookFavouriteButton userBook={makeBook()} />)

    // Start not pressed
    expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(screen.getByRole('button'))

    await waitFor(() => {
      // After rejection, should revert to not pressed
      expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'false')
    })
  })
})
