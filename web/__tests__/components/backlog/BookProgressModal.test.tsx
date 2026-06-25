import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookProgressModal from '@/components/backlog/BookProgressModal'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateProgress = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateProgress: () => mockUpdateProgress
}))

function makeBook(overrides: Parameters<typeof create<typeof UserBookSchema>>[1] = {}) {
  return create(UserBookSchema, {
    id: 'ub-1',
    status: 'currently-reading',
    tags: [],
    formats: [],
    progressMode: 'pages',
    currentPage: 50,
    progressPercent: 0,
    book: create(BookSchema, { title: 'Dune', authors: ['Frank Herbert'], pageCount: 300 }),
    ...overrides
  })
}

describe('BookProgressModal', () => {
  beforeEach(() => {
    mockUpdateProgress.mockReset()
    mockUpdateProgress.mockResolvedValue(undefined)
  })

  it('renders the book title', () => {
    render(<BookProgressModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('defaults to stored progress mode when set', () => {
    render(
      <BookProgressModal
        userBook={makeBook({ progressMode: 'pages' })}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const modeSelect = screen.getByLabelText('Progress') as HTMLSelectElement
    expect(modeSelect.value).toBe('pages')
    // Should show the pages input
    expect(screen.getByLabelText('Current page')).toBeInTheDocument()
  })

  it('defaults to percent for digital-only when no stored mode', () => {
    render(
      <BookProgressModal
        userBook={makeBook({ progressMode: '', tags: ['own-digital'] })}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const modeSelect = screen.getByLabelText('Progress') as HTMLSelectElement
    expect(modeSelect.value).toBe('percent')
    expect(screen.getByLabelText('Progress percent')).toBeInTheDocument()
  })

  it('defaults to pages for physical books when no stored mode', () => {
    render(
      <BookProgressModal
        userBook={makeBook({ progressMode: '', tags: ['own-physical'] })}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const modeSelect = screen.getByLabelText('Progress') as HTMLSelectElement
    expect(modeSelect.value).toBe('pages')
    expect(screen.getByLabelText('Current page')).toBeInTheDocument()
  })

  it('pre-fills current page in pages mode', () => {
    render(
      <BookProgressModal
        userBook={makeBook({ progressMode: 'pages', currentPage: 120 })}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const pageInput = screen.getByLabelText('Current page') as HTMLInputElement
    expect(pageInput.value).toBe('120')
  })

  it('shows total page count hint in pages mode', () => {
    render(<BookProgressModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    expect(screen.getByText('/ 300')).toBeInTheDocument()
  })

  it('pre-fills percent in percent mode', () => {
    render(
      <BookProgressModal
        userBook={makeBook({ progressMode: 'percent', progressPercent: 75 })}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const percentInput = screen.getByLabelText('Progress percent') as HTMLInputElement
    expect(percentInput.value).toBe('75')
  })

  it('calls onClose when Cancel clicked', () => {
    const onClose = jest.fn()
    render(<BookProgressModal userBook={makeBook()} onClose={onClose} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls updateProgress and onSaved on save', async () => {
    const onSaved = jest.fn()
    const onClose = jest.fn()
    render(<BookProgressModal userBook={makeBook()} onClose={onClose} onSaved={onSaved} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'ub-1',
        progressMode: 'pages',
        currentPage: 50,
        progressPercent: 0
      })
      expect(onSaved).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('saves updated page value', async () => {
    render(<BookProgressModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Current page'), { target: { value: '180' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(
        expect.objectContaining({ currentPage: 180, progressMode: 'pages' })
      )
    })
  })

  it('switches to percent mode and saves percent value', async () => {
    render(<BookProgressModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Progress'), { target: { value: 'percent' } })
    fireEvent.change(screen.getByLabelText('Progress percent'), { target: { value: '65' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith(
        expect.objectContaining({ progressMode: 'percent', progressPercent: 65 })
      )
    })
  })

  it('shows an error when updateProgress throws', async () => {
    mockUpdateProgress.mockRejectedValue(new Error('Progress save failed'))
    render(<BookProgressModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(screen.getByText('Progress save failed')).toBeInTheDocument()
    })
  })
})
