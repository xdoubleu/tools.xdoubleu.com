import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookEditModal from '@/components/backlog/BookEditModal'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateBookStatus = jest.fn()
const mockToggleTag = jest.fn()
const mockUpdateProgress = jest.fn()
const mockEnableKoboSync = jest.fn()
const mockRequestConversion = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus,
  useToggleTag: () => mockToggleTag,
  useUpdateProgress: () => mockUpdateProgress,
  useEnableKoboSync: () => mockEnableKoboSync,
  useKEPUBStatus: () => ({ data: { hasEpub: false, kepubStatus: '' } }),
  useRequestKEPUBConversion: () => () => mockRequestConversion(),
  useGetBookFile: () => ({ data: null, error: null })
}))

const userBook = create(UserBookSchema, {
  id: 'ub-1',
  status: 'reading',
  rating: 3,
  notes: 'Great so far',
  tags: ['favourite'],
  progressMode: 'pages',
  currentPage: 50,
  book: create(BookSchema, {
    title: 'Clean Code',
    authors: ['Robert C. Martin'],
    pageCount: 200
  })
})

describe('BookEditModal', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockToggleTag.mockReset()
    mockUpdateProgress.mockReset()
    mockUpdateProgress.mockResolvedValue(undefined)
  })

  it('renders book title and authors', () => {
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    expect(screen.getByText('Clean Code')).toBeInTheDocument()
    expect(screen.getByText('Robert C. Martin')).toBeInTheDocument()
  })

  it('pre-fills status from userBook', () => {
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    const statusSelect = screen.getByLabelText('Status') as HTMLSelectElement
    expect(statusSelect.value).toBe('reading')
  })

  it('pre-fills notes from userBook', () => {
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    const notes = screen.getByLabelText('Notes') as HTMLTextAreaElement
    expect(notes.value).toBe('Great so far')
  })

  it('favourite checkbox is checked when tag includes favourite', () => {
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    const checkbox = screen.getByLabelText('Favourite') as HTMLInputElement
    expect(checkbox.checked).toBe(true)
  })

  it('calls onClose when Cancel clicked', () => {
    const onClose = jest.fn()
    render(<BookEditModal userBook={userBook} onClose={onClose} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls updateBookStatus and onSaved on save', async () => {
    const onSaved = jest.fn()
    const onClose = jest.fn()
    mockUpdateBookStatus.mockResolvedValue(undefined)
    mockToggleTag.mockResolvedValue(undefined)
    render(<BookEditModal userBook={userBook} onClose={onClose} onSaved={onSaved} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalled()
      expect(onSaved).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('shows error when updateBookStatus throws', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('Save failed'))
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(screen.getByText('Save failed')).toBeInTheDocument()
    })
  })

  it('toggles own-physical tag when checkbox changed', async () => {
    mockUpdateBookStatus.mockResolvedValue(undefined)
    mockToggleTag.mockResolvedValue(undefined)
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.click(screen.getByLabelText('Own physical'))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'own-physical')
    })
  })

  it('pre-fills the current page in pages mode and shows the total', () => {
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    const page = screen.getByLabelText('Current page') as HTMLInputElement
    expect(page.value).toBe('50')
    expect(screen.getByText('/ 200')).toBeInTheDocument()
  })

  it('saves updated page progress', async () => {
    mockUpdateBookStatus.mockResolvedValue(undefined)
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Current page'), { target: { value: '120' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'ub-1',
        progressMode: 'pages',
        currentPage: 120,
        progressPercent: 0
      })
    })
  })

  it('switches to percent mode and saves a percent value', async () => {
    mockUpdateBookStatus.mockResolvedValue(undefined)
    render(<BookEditModal userBook={userBook} onClose={jest.fn()} onSaved={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Progress'), { target: { value: 'percent' } })
    fireEvent.change(screen.getByLabelText('Progress percent'), { target: { value: '75' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateProgress).toHaveBeenCalledWith({
        bookId: 'ub-1',
        progressMode: 'percent',
        currentPage: 50,
        progressPercent: 75
      })
    })
  })

  describe('Preview buttons', () => {
    const pdfOnlyBook = create(UserBookSchema, {
      ...userBook,
      formats: ['pdf']
    })

    const epubAndPdfBook = create(UserBookSchema, {
      ...userBook,
      formats: ['epub', 'pdf']
    })

    it('shows "Preview PDF" when book has pdf format', () => {
      render(<BookEditModal userBook={pdfOnlyBook} onClose={jest.fn()} onSaved={jest.fn()} />)
      expect(screen.getByRole('button', { name: 'Preview PDF' })).toBeInTheDocument()
    })

    it('shows "Preview EPUB" for a PDF-only book (triggers on-demand conversion)', () => {
      render(<BookEditModal userBook={pdfOnlyBook} onClose={jest.fn()} onSaved={jest.fn()} />)
      expect(screen.getByRole('button', { name: 'Preview EPUB' })).toBeInTheDocument()
    })

    it('shows "Preview EPUB" pointing at the native epub when book has epub format', () => {
      render(<BookEditModal userBook={epubAndPdfBook} onClose={jest.fn()} onSaved={jest.fn()} />)
      // Both buttons present; EPUB button from native epub (not kepub conversion).
      expect(screen.getByRole('button', { name: 'Preview PDF' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Preview EPUB' })).toBeInTheDocument()
    })

    it('does not show "Preview EPUB" when book has no pdf or epub', () => {
      const noFormatsBook = create(UserBookSchema, { ...userBook, formats: [] })
      render(<BookEditModal userBook={noFormatsBook} onClose={jest.fn()} onSaved={jest.fn()} />)
      expect(screen.queryByRole('button', { name: 'Preview EPUB' })).not.toBeInTheDocument()
    })
  })
})
