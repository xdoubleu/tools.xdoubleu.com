import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateProgress = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateProgress: () => mockUpdateProgress
}))

jest.mock('@/components/backlog/BookProgressBar', () => {
  return function MockProgressBar() {
    return <div role="progressbar" data-testid="progress-bar" />
  }
})

import BookProgressEditor from '@/components/backlog/BookProgressEditor'

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
    status: 'currently-reading',
    progressMode: overrides.progressMode ?? 'pages',
    currentPage: overrides.currentPage ?? 50,
    progressPercent: overrides.progressPercent ?? 0,
    tags: overrides.tags ?? [],
    formats: [],
    book: create(BookSchema, {
      title: 'Test Book',
      authors: ['Author'],
      pageCount: overrides.pageCount ?? 200
    })
  })
}

describe('BookProgressEditor', () => {
  beforeEach(() => {
    mockUpdateProgress.mockReset()
    mockMutate.mockReset()
    mockUpdateProgress.mockResolvedValue({})
  })

  it('renders the progress bar in read-only mode initially', () => {
    render(<BookProgressEditor userBook={makeBook()} />)
    expect(screen.getByTestId('progress-bar')).toBeInTheDocument()
    expect(screen.queryByLabelText('Current page')).not.toBeInTheDocument()
  })

  it('shows the edit form when the progress bar is clicked', () => {
    render(<BookProgressEditor userBook={makeBook()} />)
    fireEvent.click(screen.getByLabelText('Edit reading progress'))
    expect(screen.getByLabelText('Current page')).toBeInTheDocument()
  })

  it('commits pages progress on Enter and calls UpdateProgress', async () => {
    render(<BookProgressEditor userBook={makeBook({ progressMode: 'pages', currentPage: 50 })} />)
    fireEvent.click(screen.getByLabelText('Edit reading progress'))

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '120' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'ub-1',
        progressMode: 'pages',
        currentPage: 120,
        progressPercent: 0
      })
    })
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('commits percent progress on blur', async () => {
    render(
      <BookProgressEditor
        userBook={makeBook({ progressMode: 'percent', progressPercent: 20, tags: ['own-digital'] })}
      />
    )
    fireEvent.click(screen.getByLabelText('Edit reading progress'))

    const input = screen.getByLabelText('Progress percent')
    fireEvent.change(input, { target: { value: '75' } })
    fireEvent.blur(input)

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(
        expect.objectContaining({ progressPercent: 75 })
      )
    })
  })

  it('closes editor after successful save', async () => {
    render(<BookProgressEditor userBook={makeBook()} />)
    fireEvent.click(screen.getByLabelText('Edit reading progress'))
    expect(screen.getByLabelText('Current page')).toBeInTheDocument()

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(screen.queryByLabelText('Current page')).not.toBeInTheDocument()
    })
  })

  it('pressing Escape cancels editing without saving', () => {
    render(<BookProgressEditor userBook={makeBook({ currentPage: 50 })} />)
    fireEvent.click(screen.getByLabelText('Edit reading progress'))

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '99' } })
    fireEvent.keyDown(input, { key: 'Escape' })

    expect(screen.queryByLabelText('Current page')).not.toBeInTheDocument()
    expect(mockUpdateProgress).not.toHaveBeenCalled()
  })

  it('calls onSaved after successful save', async () => {
    const onSaved = jest.fn()
    render(<BookProgressEditor userBook={makeBook()} onSaved={onSaved} />)
    fireEvent.click(screen.getByLabelText('Edit reading progress'))
    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })
})
