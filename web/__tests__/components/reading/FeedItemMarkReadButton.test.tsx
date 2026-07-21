import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/reading/v1/library_pb'

const mockUpdateBookStatus = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBooks', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus
}))

import FeedItemMarkReadButton from '@/components/reading/FeedItemMarkReadButton'

function makeUserBook(status = 'to-read', tags: string[] = [], rating = 0) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status,
    rating,
    tags,
    formats: [],
    book: create(BookSchema, { title: 'Test Article', authors: [] })
  })
}

describe('FeedItemMarkReadButton', () => {
  beforeEach(() => {
    jest.useFakeTimers()
    mockUpdateBookStatus.mockReset()
    mockMutate.mockReset()
    mockUpdateBookStatus.mockResolvedValue({})
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('renders a "Mark read" button', () => {
    render(<FeedItemMarkReadButton userBook={makeUserBook()} onSettled={jest.fn()} />)
    expect(screen.getByRole('button', { name: 'Mark read' })).toBeInTheDocument()
  })

  it('marks the item read and shows an Undo affordance', async () => {
    render(<FeedItemMarkReadButton userBook={makeUserBook()} onSettled={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith({
        bookId: 'book-1',
        status: 'read',
        favourite: false,
        rating: '0'
      })
    })
    expect(mockMutate).toHaveBeenCalledWith('/reading')
    expect(screen.getByRole('button', { name: 'Undo' })).toBeInTheDocument()
  })

  it('calls onSettled once the undo window elapses', async () => {
    const onSettled = jest.fn()
    render(<FeedItemMarkReadButton userBook={makeUserBook()} onSettled={onSettled} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    await waitFor(() => screen.getByRole('button', { name: 'Undo' }))

    jest.advanceTimersByTime(4000)

    expect(onSettled).toHaveBeenCalledWith('book-1')
  })

  it('reverts to the prior status and never settles when Undo is clicked', async () => {
    const onSettled = jest.fn()
    render(<FeedItemMarkReadButton userBook={makeUserBook()} onSettled={onSettled} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))
    await waitFor(() => screen.getByRole('button', { name: 'Undo' }))

    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))

    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenLastCalledWith({
        bookId: 'book-1',
        status: 'to-read',
        favourite: false,
        rating: '0'
      })
    })
    expect(screen.getByRole('button', { name: 'Mark read' })).toBeInTheDocument()

    jest.advanceTimersByTime(4000)
    expect(onSettled).not.toHaveBeenCalled()
  })

  it('reverts to the button on error', async () => {
    mockUpdateBookStatus.mockRejectedValueOnce(new Error('fail'))
    render(<FeedItemMarkReadButton userBook={makeUserBook()} onSettled={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Mark read' }))

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Mark read' })).toBeInTheDocument()
    })
  })
})
