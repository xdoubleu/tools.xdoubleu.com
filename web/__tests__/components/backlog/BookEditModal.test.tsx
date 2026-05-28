import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookEditModal from '@/components/backlog/BookEditModal'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateBookStatus = jest.fn()
const mockToggleTag = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus,
  useToggleTag: () => mockToggleTag
}))

const userBook = create(UserBookSchema, {
  id: 'ub-1',
  status: 'reading',
  rating: 3,
  notes: 'Great so far',
  tags: ['favourite'],
  book: create(BookSchema, {
    title: 'Clean Code',
    authors: ['Robert C. Martin']
  })
})

describe('BookEditModal', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockToggleTag.mockReset()
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
})
