import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

const mockUpdateProgress = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: jest.fn()
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

  it('renders the progress bar', () => {
    render(<BookQuickProgress userBook={makeBook()} />)
    expect(screen.getByText('50 / 200 pages')).toBeInTheDocument()
  })

  it('does not render step buttons', () => {
    render(<BookQuickProgress userBook={makeBook()} />)
    expect(screen.queryByLabelText(/Increase progress/)).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/Decrease progress/)).not.toBeInTheDocument()
  })

  it('opens the exact-entry form when the bar is tapped', () => {
    render(<BookQuickProgress userBook={makeBook()} />)

    fireEvent.click(screen.getByLabelText('Edit reading progress'))

    expect(screen.getByLabelText('Current page')).toBeInTheDocument()
  })

  it('calls onSaved after committing an edited value', async () => {
    const onSaved = jest.fn()
    render(<BookQuickProgress userBook={makeBook()} onSaved={onSaved} />)

    fireEvent.click(screen.getByLabelText('Edit reading progress'))
    fireEvent.change(screen.getByLabelText('Current page'), { target: { value: '75' } })
    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })
})
