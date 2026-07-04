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

import BookRatingStars from '@/components/books/BookRatingStars'

function makeBook(rating = 0, tags: string[] = []) {
  return create(UserBookSchema, {
    id: 'ub-1',
    status: 'to-read',
    rating,
    tags,
    formats: [],
    book: create(BookSchema, { title: 'Test Book', authors: ['Author'] })
  })
}

describe('BookRatingStars', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockMutate.mockReset()
    mockUpdateBookStatus.mockResolvedValue({})
  })

  it('renders 5 star buttons', () => {
    render(<BookRatingStars userBook={makeBook()} />)
    expect(screen.getAllByRole('button').length).toBe(5)
  })

  it('fires UpdateBookStatus with new rating on click', async () => {
    const ub = makeBook(0, ['favourite'])
    render(<BookRatingStars userBook={ub} />)

    fireEvent.click(screen.getByLabelText('Rate 4 stars'))

    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith({
        bookId: 'ub-1',
        status: 'to-read',
        favourite: true,
        rating: '4'
      })
    })
    expect(mockMutate).toHaveBeenCalledWith('/books')
  })

  it('clears rating when clicking the current rating', async () => {
    render(<BookRatingStars userBook={makeBook(3)} />)

    fireEvent.click(screen.getByLabelText('Rate 3 stars'))

    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(expect.objectContaining({ rating: '0' }))
    })
  })

  it('calls onSaved after successful save', async () => {
    const onSaved = jest.fn()
    render(<BookRatingStars userBook={makeBook()} onSaved={onSaved} />)

    fireEvent.click(screen.getByLabelText('Rate 2 stars'))

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('reverts to previous rating on error', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('network error'))
    render(<BookRatingStars userBook={makeBook(3)} />)

    // Click to change to 5
    fireEvent.click(screen.getByLabelText('Rate 5 stars'))

    await waitFor(() => {
      // After rejection, rating stays at 3 (reverted)
      expect(screen.getByLabelText('3 out of 5 stars')).toBeInTheDocument()
    })
  })

  it('does not fire click handler when readOnly', () => {
    render(<BookRatingStars userBook={makeBook(2)} readOnly />)

    fireEvent.click(screen.getAllByRole('button')[0])
    expect(mockUpdateBookStatus).not.toHaveBeenCalled()
  })
})
