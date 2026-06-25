import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookEntryModal from '@/components/backlog/BookEntryModal'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateBookStatus = jest.fn()
const mockToggleTag = jest.fn()
const mockEnableKoboSync = jest.fn()
const mockRequestConversion = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus,
  useToggleTag: () => mockToggleTag,
  useEnableKoboSync: () => mockEnableKoboSync,
  useKEPUBStatus: () => ({ data: { hasEpub: false, kepubStatus: '' } }),
  useRequestKEPUBConversion: () => () => mockRequestConversion(),
  useGetBookFile: () => ({ data: null, error: null })
}))

function makeBook(overrides: Parameters<typeof create<typeof UserBookSchema>>[1] = {}) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status: 'currently-reading',
    rating: 3,
    notes: 'Great so far',
    tags: ['favourite'],
    formats: [],
    progressMode: 'pages',
    currentPage: 50,
    book: create(BookSchema, {
      title: 'Clean Code',
      authors: ['Robert C. Martin'],
      pageCount: 200
    }),
    ...overrides
  })
}

describe('BookEntryModal', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockToggleTag.mockReset()
    mockUpdateBookStatus.mockResolvedValue(undefined)
    mockToggleTag.mockResolvedValue(undefined)
  })

  it('renders book title and authors', () => {
    render(<BookEntryModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    expect(screen.getByText('Clean Code')).toBeInTheDocument()
    expect(screen.getByText('Robert C. Martin')).toBeInTheDocument()
  })

  it('pre-fills rating from userBook', () => {
    render(<BookEntryModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    const rating = screen.getByLabelText('Rating') as HTMLSelectElement
    expect(rating.value).toBe('3')
  })

  it('pre-fills notes from userBook', () => {
    render(<BookEntryModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    const notes = screen.getByLabelText('Notes') as HTMLTextAreaElement
    expect(notes.value).toBe('Great so far')
  })

  it('favourite checkbox is checked when tag includes favourite', () => {
    render(<BookEntryModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    const checkbox = screen.getByLabelText('Favourite') as HTMLInputElement
    expect(checkbox.checked).toBe(true)
  })

  it('own-physical checkbox reflects tag', () => {
    render(
      <BookEntryModal
        userBook={makeBook({ tags: ['own-physical'] })}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const checkbox = screen.getByLabelText('Own physical') as HTMLInputElement
    expect(checkbox.checked).toBe(true)
  })

  it('own-digital checkbox reflects tag', () => {
    render(
      <BookEntryModal
        userBook={makeBook({ tags: ['own-digital'] })}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const checkbox = screen.getByLabelText('Own digital') as HTMLInputElement
    expect(checkbox.checked).toBe(true)
  })

  it('calls onClose when Cancel clicked', () => {
    const onClose = jest.fn()
    render(<BookEntryModal userBook={makeBook()} onClose={onClose} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls updateBookStatus and onSaved on save', async () => {
    const onSaved = jest.fn()
    const onClose = jest.fn()
    render(<BookEntryModal userBook={makeBook()} onClose={onClose} onSaved={onSaved} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalled()
      expect(onSaved).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('passes through the current status unchanged', async () => {
    const ub = makeBook({ status: 'currently-reading' })
    render(<BookEntryModal userBook={ub} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'currently-reading' })
      )
    })
  })

  it('shows error when updateBookStatus throws', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('Save failed'))
    render(<BookEntryModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(screen.getByText('Save failed')).toBeInTheDocument()
    })
  })

  it('toggles own-physical tag when checkbox changed', async () => {
    render(<BookEntryModal userBook={makeBook()} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByLabelText('Own physical'))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'own-physical')
    })
  })

  describe('Preview buttons', () => {
    it('shows "Preview PDF" when book has pdf format', () => {
      render(
        <BookEntryModal
          userBook={makeBook({ formats: ['pdf'] })}
          onClose={jest.fn()}
          onSaved={jest.fn()}
        />
      )
      expect(screen.getByRole('button', { name: 'Preview PDF' })).toBeInTheDocument()
    })

    it('shows "Preview EPUB" for a PDF-only book (on-demand conversion)', () => {
      render(
        <BookEntryModal
          userBook={makeBook({ formats: ['pdf'] })}
          onClose={jest.fn()}
          onSaved={jest.fn()}
        />
      )
      expect(screen.getByRole('button', { name: 'Preview EPUB' })).toBeInTheDocument()
    })

    it('shows both Preview buttons when book has epub and pdf', () => {
      render(
        <BookEntryModal
          userBook={makeBook({ formats: ['epub', 'pdf'] })}
          onClose={jest.fn()}
          onSaved={jest.fn()}
        />
      )
      expect(screen.getByRole('button', { name: 'Preview PDF' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Preview EPUB' })).toBeInTheDocument()
    })

    it('does not show preview buttons when book has no files', () => {
      render(
        <BookEntryModal
          userBook={makeBook({ formats: [] })}
          onClose={jest.fn()}
          onSaved={jest.fn()}
        />
      )
      expect(screen.queryByRole('button', { name: 'Preview PDF' })).not.toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Preview EPUB' })).not.toBeInTheDocument()
    })
  })
})
