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

import BookProgressForm from '@/components/books/BookProgressForm'

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

describe('BookProgressForm', () => {
  beforeEach(() => {
    mockUpdateProgress.mockReset()
    mockMutate.mockReset()
    mockUpdateProgress.mockResolvedValue({})
  })

  it('commits pages progress on Enter and calls UpdateProgress', async () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'pages', currentPage: 50 })} />)

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
      <BookProgressForm
        userBook={makeBook({ progressMode: 'percent', progressPercent: 20, tags: ['own-digital'] })}
      />
    )

    const input = screen.getByLabelText('Progress percent')
    fireEvent.change(input, { target: { value: '75' } })
    fireEvent.blur(input)

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(
        expect.objectContaining({ progressPercent: 75 })
      )
    })
  })

  it('switches the input to percent mode when the mode select changes', () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'pages' })} />)

    fireEvent.change(screen.getByLabelText('Progress mode'), { target: { value: 'percent' } })

    expect(screen.getByLabelText('Progress percent')).toBeInTheDocument()
    expect(screen.queryByLabelText('Current page')).not.toBeInTheDocument()
  })

  it('commits pages progress on blur', async () => {
    render(<BookProgressForm userBook={makeBook({ progressMode: 'pages', currentPage: 50 })} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '75' } })
    fireEvent.blur(input)

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(expect.objectContaining({ currentPage: 75 }))
    })
  })

  it('calls onClose after a successful save', async () => {
    const onClose = jest.fn()
    render(<BookProgressForm userBook={makeBook()} onClose={onClose} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('calls onSaved after a successful save', async () => {
    const onSaved = jest.fn()
    render(<BookProgressForm userBook={makeBook()} onSaved={onSaved} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('pressing Escape calls onClose and resets without saving', () => {
    const onClose = jest.fn()
    render(<BookProgressForm userBook={makeBook({ currentPage: 50 })} onClose={onClose} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.change(input, { target: { value: '99' } })
    fireEvent.keyDown(input, { key: 'Escape' })

    expect(onClose).toHaveBeenCalled()
    expect(mockUpdateProgress).not.toHaveBeenCalled()
  })

  it('pressing Escape without an onClose handler does not throw', () => {
    render(<BookProgressForm userBook={makeBook({ currentPage: 50 })} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Escape' })

    expect(mockUpdateProgress).not.toHaveBeenCalled()
  })

  it('ignores a second commit while a save is already in flight', async () => {
    let resolveUpdate: () => void = () => {}
    mockUpdateProgress.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveUpdate = () => resolve({})
      })
    )
    render(<BookProgressForm userBook={makeBook()} />)

    const input = screen.getByLabelText('Current page')
    fireEvent.keyDown(input, { key: 'Enter' })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(mockUpdateProgress).toHaveBeenCalledTimes(1)
    resolveUpdate()
    await waitFor(() => expect(mockUpdateProgress).toHaveBeenCalledTimes(1))
  })

  it('omits the page-count suffix when the book has no page count', () => {
    render(<BookProgressForm userBook={makeBook({ pageCount: 0 })} />)

    expect(screen.queryByText(/^\/ /)).not.toBeInTheDocument()
  })

  it('keeps the form open if the save fails', async () => {
    mockUpdateProgress.mockRejectedValueOnce(new Error('network error'))
    const onClose = jest.fn()
    render(<BookProgressForm userBook={makeBook()} onClose={onClose} />)

    fireEvent.keyDown(screen.getByLabelText('Current page'), { key: 'Enter' })

    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalled()
    })
    expect(onClose).not.toHaveBeenCalled()
  })

  it('selects the existing value on focus so retyping does not require clearing it first', () => {
    render(<BookProgressForm userBook={makeBook({ currentPage: 50 })} />)

    const input = screen.getByLabelText('Current page') as HTMLInputElement
    const selectSpy = jest.spyOn(input, 'select')
    fireEvent.focus(input)

    expect(selectSpy).toHaveBeenCalled()
  })
})
