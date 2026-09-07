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

import MarkAsCompletedDialog from '@/components/books/MarkAsCompletedDialog'

function makeBook(overrides: { rating?: number; tags?: string[] } = {}) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status: 'currently-reading',
    rating: overrides.rating ?? 0,
    tags: overrides.tags ?? [],
    book: create(BookSchema, { title: 'A Book' })
  })
}

describe('MarkAsCompletedDialog', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUpdateBookStatus.mockResolvedValue({})
  })

  it('marks the book as read with no rating or favourite by default', async () => {
    render(<MarkAsCompletedDialog userBook={makeBook()} open={true} onOpenChange={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark as completed' }))

    await waitFor(() =>
      expect(mockUpdateBookStatus).toHaveBeenCalledWith({
        bookId: 'book-1',
        status: 'read',
        favourite: false,
        rating: '0'
      })
    )
    expect(mockMutate).toHaveBeenCalled()
  })

  it('includes a chosen rating and favourite toggle in the completion call', async () => {
    const onOpenChange = jest.fn()
    const onCompleted = jest.fn()
    render(
      <MarkAsCompletedDialog
        userBook={makeBook()}
        open={true}
        onOpenChange={onOpenChange}
        onCompleted={onCompleted}
      />
    )

    fireEvent.click(screen.getByLabelText('Rate 4 stars'))
    fireEvent.click(screen.getByLabelText('Add to favourites'))
    fireEvent.click(screen.getByRole('button', { name: 'Mark as completed' }))

    await waitFor(() =>
      expect(mockUpdateBookStatus).toHaveBeenCalledWith({
        bookId: 'book-1',
        status: 'read',
        favourite: true,
        rating: '4'
      })
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onCompleted).toHaveBeenCalled()
  })

  it('shows an error and keeps the dialog open when the save fails', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('nope'))
    const onOpenChange = jest.fn()
    render(<MarkAsCompletedDialog userBook={makeBook()} open={true} onOpenChange={onOpenChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark as completed' }))

    expect(await screen.findByTestId('mark-completed-error')).toBeInTheDocument()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('highlights hovered stars and clears the highlight on mouse leave', () => {
    render(<MarkAsCompletedDialog userBook={makeBook()} open={true} onOpenChange={jest.fn()} />)

    const star3 = screen.getByLabelText('Rate 3 stars')
    fireEvent.mouseEnter(star3)
    expect(star3).toHaveClass('text-amber-400')

    fireEvent.mouseLeave(star3.parentElement!)
    expect(star3).toHaveClass('text-border')
  })

  it('resets the chosen rating and favourite when cancelled', () => {
    const onOpenChange = jest.fn()
    render(<MarkAsCompletedDialog userBook={makeBook()} open={true} onOpenChange={onOpenChange} />)

    fireEvent.click(screen.getByLabelText('Rate 4 stars'))
    fireEvent.click(screen.getByLabelText('Add to favourites'))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(screen.getByLabelText('No rating')).toBeInTheDocument()
    expect(screen.getByLabelText('Add to favourites')).toBeInTheDocument()
  })
})
