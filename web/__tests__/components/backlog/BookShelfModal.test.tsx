import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import BookShelfModal from '@/components/backlog/BookShelfModal'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateBookStatus = jest.fn()
const mockToggleTag = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus,
  useToggleTag: () => mockToggleTag
}))

function makeBook(overrides: Parameters<typeof create<typeof UserBookSchema>>[1] = {}) {
  return create(UserBookSchema, {
    id: 'ub-1',
    status: 'currently-reading',
    rating: 3,
    notes: 'Good',
    tags: ['favourite'],
    formats: [],
    progressMode: 'pages',
    book: create(BookSchema, { title: 'Dune', authors: ['Frank Herbert'] }),
    ...overrides
  })
}

describe('BookShelfModal', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockToggleTag.mockReset()
    mockUpdateBookStatus.mockResolvedValue(undefined)
    mockToggleTag.mockResolvedValue(undefined)
  })

  it('renders the book title', () => {
    render(
      <BookShelfModal
        userBook={makeBook()}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    expect(screen.getByText('Dune')).toBeInTheDocument()
  })

  it('pre-fills the canonical status from userBook', () => {
    render(
      <BookShelfModal
        userBook={makeBook({ status: 'currently-reading' })}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const select = screen.getByLabelText('Status') as HTMLSelectElement
    expect(select.value).toBe('currently-reading')
  })

  it('preselects "to-read" status correctly', () => {
    render(
      <BookShelfModal
        userBook={makeBook({ status: 'to-read' })}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const select = screen.getByLabelText('Status') as HTMLSelectElement
    expect(select.value).toBe('to-read')
  })

  it('preselects "read" status correctly', () => {
    render(
      <BookShelfModal
        userBook={makeBook({ status: 'read' })}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const select = screen.getByLabelText('Status') as HTMLSelectElement
    expect(select.value).toBe('read')
  })

  it('preselects "dropped" status correctly', () => {
    render(
      <BookShelfModal
        userBook={makeBook({ status: 'dropped' })}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    const select = screen.getByLabelText('Status') as HTMLSelectElement
    expect(select.value).toBe('dropped')
  })

  it('shows existing custom shelf tags as chips', () => {
    render(
      <BookShelfModal
        userBook={makeBook({ tags: ['sci-fi', 'favourite'] })}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    // sci-fi is a custom shelf; favourite is special and should not appear
    expect(screen.getByText('sci-fi')).toBeInTheDocument()
    expect(screen.queryByText('favourite')).not.toBeInTheDocument()
  })

  it('calls onClose when Cancel clicked', () => {
    const onClose = jest.fn()
    render(
      <BookShelfModal
        userBook={makeBook()}
        knownShelves={[]}
        onClose={onClose}
        onSaved={jest.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('calls updateBookStatus with canonical status on save', async () => {
    const onSaved = jest.fn()
    const onClose = jest.fn()
    render(
      <BookShelfModal
        userBook={makeBook({ status: 'currently-reading' })}
        knownShelves={[]}
        onClose={onClose}
        onSaved={onSaved}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'currently-reading' })
      )
      expect(onSaved).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('passes through rating, notes, and favourite so they are not clobbered', async () => {
    render(
      <BookShelfModal
        userBook={makeBook({ rating: 5, notes: 'My notes', tags: ['favourite'] })}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({
          rating: '5',
          notes: 'My notes',
          favourite: true
        })
      )
    })
  })

  it('toggles a new shelf tag via Add button', async () => {
    render(
      <BookShelfModal
        userBook={makeBook()}
        knownShelves={['classics']}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    fireEvent.change(screen.getByLabelText('Shelf name'), { target: { value: 'classics' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'classics')
    })
  })

  it('removes a shelf chip and toggles the tag on save', async () => {
    render(
      <BookShelfModal
        userBook={makeBook({ tags: ['sci-fi'] })}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    fireEvent.click(screen.getByLabelText('Remove shelf sci-fi'))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'sci-fi')
    })
  })

  it('shows an error when save fails', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('Network error'))
    render(
      <BookShelfModal
        userBook={makeBook()}
        knownShelves={[]}
        onClose={jest.fn()}
        onSaved={jest.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })
})
