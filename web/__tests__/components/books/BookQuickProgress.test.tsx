import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

const mockUpdateProgress = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBooks', () => ({
  useUpdateProgress: () => mockUpdateProgress
}))

import BookQuickProgress from '@/components/books/BookQuickProgress'

function makeBook(
  overrides: {
    progressMode?: string
    currentPage?: number
    progressPercent?: number
    tags?: string[]
    pageCount?: number
  } = {}
) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status: 'currently-reading',
    progressMode: overrides.progressMode ?? 'pages',
    currentPage: overrides.currentPage ?? 50,
    progressPercent: overrides.progressPercent ?? 25,
    tags: overrides.tags ?? [],
    book: create(BookSchema, {
      title: 'A Book',
      pageCount: overrides.pageCount ?? 200
    })
  })
}

describe('BookQuickProgress', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUpdateProgress.mockResolvedValue({})
  })

  it('steps pages up by 10 and revalidates the library', async () => {
    render(<BookQuickProgress userBook={makeBook()} />)

    fireEvent.click(screen.getByLabelText('Increase progress by 10 pages'))

    await waitFor(() =>
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'book-1',
        progressMode: 'pages',
        currentPage: 60,
        progressPercent: 25
      })
    )
    expect(mockMutate).toHaveBeenCalled()
    expect(await screen.findByText('60 / 200 pages')).toBeInTheDocument()
  })

  it('steps percent by 5 in percent mode', async () => {
    render(<BookQuickProgress userBook={makeBook({ progressMode: 'percent' })} />)

    fireEvent.click(screen.getByLabelText('Decrease progress by 5 percent'))

    await waitFor(() =>
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'book-1',
        progressMode: 'percent',
        currentPage: 50,
        progressPercent: 20
      })
    )
  })

  it('clamps at the page count and at zero', async () => {
    render(<BookQuickProgress userBook={makeBook({ currentPage: 195 })} />)

    fireEvent.click(screen.getByLabelText('Increase progress by 10 pages'))
    await waitFor(() =>
      expect(mockUpdateProgress).toHaveBeenCalledWith(expect.objectContaining({ currentPage: 200 }))
    )

    fireEvent.click(screen.getByLabelText('Increase progress by 10 pages'))
    expect(mockUpdateProgress).toHaveBeenCalledTimes(1)
  })

  it('reverts the optimistic value when the save fails', async () => {
    mockUpdateProgress.mockRejectedValue(new Error('nope'))
    render(<BookQuickProgress userBook={makeBook()} />)

    fireEvent.click(screen.getByLabelText('Increase progress by 10 pages'))

    await waitFor(() => expect(screen.getByText('50 / 200 pages')).toBeInTheDocument())
  })

  it('hides the steppers in pages mode without a page count', () => {
    render(<BookQuickProgress userBook={makeBook({ pageCount: 0 })} />)

    expect(screen.queryByLabelText('Increase progress by 10 pages')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Edit reading progress')).toBeInTheDocument()
  })

  it('opens the exact-entry form when the bar is tapped', () => {
    render(<BookQuickProgress userBook={makeBook()} />)

    fireEvent.click(screen.getByLabelText('Edit reading progress'))

    expect(screen.getByLabelText('Current page')).toBeInTheDocument()
  })

  it('calls onSaved after a successful step', async () => {
    const onSaved = jest.fn()
    render(<BookQuickProgress userBook={makeBook()} onSaved={onSaved} />)

    fireEvent.click(screen.getByLabelText('Increase progress by 10 pages'))

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })
})
