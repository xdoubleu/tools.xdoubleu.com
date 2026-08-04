import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import RemoveBookDialog from '@/components/books/RemoveBookDialog'
import { UserBookSchema, type UserBook } from '@/lib/gen/books/v1/library_pb'

const mockRemoveBook = jest.fn()
const mockUpdateBookStatus = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useRemoveBook: () => mockRemoveBook,
  useUpdateBookStatus: () => mockUpdateBookStatus
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

function makeUserBook(overrides: MessageInitShape<typeof UserBookSchema> = {}): UserBook {
  return create(UserBookSchema, {
    id: '1',
    bookId: 'book-1',
    status: 'to-read',
    tags: [],
    formats: [],
    addedAt: '2024-01-01T00:00:00Z',
    ...overrides
  })
}

function renderDialog({
  userBook = makeUserBook(),
  open = true,
  onOpenChange = jest.fn(),
  onRemoved = jest.fn(),
  title = 'My Book'
}: {
  userBook?: UserBook
  open?: boolean
  onOpenChange?: (open: boolean) => void
  onRemoved?: () => void
  title?: string
} = {}) {
  return render(
    <RemoveBookDialog
      userBook={userBook}
      title={title}
      open={open}
      onOpenChange={onOpenChange}
      onRemoved={onRemoved}
    />
  )
}

describe('RemoveBookDialog', () => {
  beforeEach(() => {
    mockRemoveBook.mockReset()
    mockUpdateBookStatus.mockReset()
    mockMutate.mockReset()
  })

  it('renders the dialog with the book title when open', () => {
    renderDialog({ title: 'My Book' })
    expect(screen.getByText('Remove from library')).toBeInTheDocument()
    expect(screen.getByText('My Book')).toBeInTheDocument()
  })

  it('does not render content when closed', () => {
    renderDialog({ open: false })
    expect(screen.queryByText('Remove from library')).not.toBeInTheDocument()
  })

  it('calls onOpenChange(false) on cancel', () => {
    const onOpenChange = jest.fn()
    renderDialog({ onOpenChange })
    fireEvent.click(screen.getByText('Cancel'))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('calls removeBook and mutate on confirm', async () => {
    mockRemoveBook.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    const onOpenChange = jest.fn()
    const onRemoved = jest.fn()
    renderDialog({ onOpenChange, onRemoved })

    fireEvent.click(screen.getByTestId('remove-book-confirm-btn'))

    await waitFor(() => expect(mockRemoveBook).toHaveBeenCalledWith('book-1'))
    expect(mockMutate).toHaveBeenCalledWith('/books')
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onRemoved).toHaveBeenCalled()
  })

  it('shows error message when removeBook fails', async () => {
    mockRemoveBook.mockRejectedValue(new Error('network error'))
    renderDialog()

    fireEvent.click(screen.getByTestId('remove-book-confirm-btn'))

    await waitFor(() => expect(screen.getByTestId('remove-book-error')).toBeInTheDocument())
    expect(screen.getByTestId('remove-book-error')).toHaveTextContent('Failed to remove book')
  })

  it('does not show the mark-owned suggestion for a non-owned book', () => {
    renderDialog({ userBook: makeUserBook({ tags: [] }) })
    expect(screen.queryByTestId('remove-book-mark-owned-btn')).not.toBeInTheDocument()
  })

  it('does not show the mark-owned suggestion when already marked owned', () => {
    renderDialog({ userBook: makeUserBook({ tags: ['own-physical'], status: 'owned' }) })
    expect(screen.queryByTestId('remove-book-mark-owned-btn')).not.toBeInTheDocument()
  })

  it('shows the mark-owned suggestion for an owned, non-owned-status book', () => {
    renderDialog({ userBook: makeUserBook({ tags: ['own-digital'], status: 'to-read' }) })
    expect(screen.getByTestId('remove-book-mark-owned-btn')).toBeInTheDocument()
  })

  it('marks the book as owned instead of removing it', async () => {
    mockUpdateBookStatus.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    const onOpenChange = jest.fn()
    const onRemoved = jest.fn()
    renderDialog({
      userBook: makeUserBook({ tags: ['own-physical'], status: 'to-read' }),
      onOpenChange,
      onRemoved
    })

    fireEvent.click(screen.getByTestId('remove-book-mark-owned-btn'))

    await waitFor(() =>
      expect(mockUpdateBookStatus).toHaveBeenCalledWith({ bookId: 'book-1', status: 'owned' })
    )
    expect(mockMutate).toHaveBeenCalledWith('/books')
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(mockRemoveBook).not.toHaveBeenCalled()
    expect(onRemoved).not.toHaveBeenCalled()
  })

  it('shows error message when marking as owned fails', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('network error'))
    renderDialog({ userBook: makeUserBook({ tags: ['own-physical'], status: 'to-read' }) })

    fireEvent.click(screen.getByTestId('remove-book-mark-owned-btn'))

    await waitFor(() => expect(screen.getByTestId('remove-book-error')).toBeInTheDocument())
    expect(screen.getByTestId('remove-book-error')).toHaveTextContent('Failed to update book')
  })
})
