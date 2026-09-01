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

jest.mock('@/components/books/BookProgressBar', () => {
  return function MockProgressBar() {
    return <div role="progressbar" data-testid="progress-bar" />
  }
})

import BookProgressEditor from '@/components/books/BookProgressEditor'

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
        bookId: 'book-1',
        progressMode: 'pages',
        currentPage: 120,
        progressPercent: 0
      })
    })
    expect(mockMutate).toHaveBeenCalledWith('/books')
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

  it('advances by one page in a single click without opening the editor', async () => {
    render(<BookProgressEditor userBook={makeBook({ currentPage: 50, pageCount: 200 })} />)

    fireEvent.click(screen.getByLabelText('Advance one page'))

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'book-1',
        progressMode: 'pages',
        currentPage: 51,
        progressPercent: 0
      })
    })
    expect(mockMutate).toHaveBeenCalledWith('/books')
    // the quick step never opens the full editor
    expect(screen.queryByLabelText('Current page')).not.toBeInTheDocument()
  })

  it('advances by one percent in a single click for percent-mode books', async () => {
    render(
      <BookProgressEditor
        userBook={makeBook({ progressMode: 'percent', progressPercent: 20, tags: ['own-digital'] })}
      />
    )

    fireEvent.click(screen.getByLabelText('Advance progress by one percent'))

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(
        expect.objectContaining({ progressPercent: 21 })
      )
    })
  })

  it('disables the quick-step button once progress is already at its max', () => {
    render(<BookProgressEditor userBook={makeBook({ currentPage: 200, pageCount: 200 })} />)
    expect(screen.getByLabelText('Advance one page')).toBeDisabled()
  })

  it('reverts the displayed value if the quick step fails to save', async () => {
    mockUpdateProgress.mockRejectedValueOnce(new Error('network error'))
    render(<BookProgressEditor userBook={makeBook({ currentPage: 50, pageCount: 200 })} />)

    fireEvent.click(screen.getByLabelText('Advance one page'))

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalled()
    })

    // reopening the editor should show the original, unchanged value
    fireEvent.click(screen.getByLabelText('Edit reading progress'))
    expect(screen.getByLabelText('Current page')).toHaveValue(50)
  })

  it('selects the existing value on focus so retyping does not require clearing it first', () => {
    render(<BookProgressEditor userBook={makeBook({ currentPage: 50 })} />)
    fireEvent.click(screen.getByLabelText('Edit reading progress'))

    const input = screen.getByLabelText('Current page') as HTMLInputElement
    const selectSpy = jest.spyOn(input, 'select')
    fireEvent.focus(input)

    expect(selectSpy).toHaveBeenCalled()
  })
})
