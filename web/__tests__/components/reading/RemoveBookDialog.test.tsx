import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import RemoveBookDialog from '@/components/reading/RemoveBookDialog'

const mockRemoveBook = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useRemoveBook: () => mockRemoveBook
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

function renderDialog(
  open = true,
  onOpenChange = jest.fn(),
  onRemoved = jest.fn(),
  title = 'My Book'
) {
  return render(
    <RemoveBookDialog
      bookId="book-1"
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
    mockMutate.mockReset()
  })

  it('renders the dialog with the book title when open', () => {
    renderDialog(true, jest.fn(), jest.fn(), 'My Book')
    expect(screen.getByText('Remove from library')).toBeInTheDocument()
    expect(screen.getByText('My Book')).toBeInTheDocument()
  })

  it('does not render content when closed', () => {
    renderDialog(false)
    expect(screen.queryByText('Remove from library')).not.toBeInTheDocument()
  })

  it('calls onOpenChange(false) on cancel', () => {
    const onOpenChange = jest.fn()
    renderDialog(true, onOpenChange)
    fireEvent.click(screen.getByText('Cancel'))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('calls removeBook and mutate on confirm', async () => {
    mockRemoveBook.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    const onOpenChange = jest.fn()
    const onRemoved = jest.fn()
    renderDialog(true, onOpenChange, onRemoved)

    fireEvent.click(screen.getByTestId('remove-book-confirm-btn'))

    await waitFor(() => expect(mockRemoveBook).toHaveBeenCalledWith('book-1'))
    expect(mockMutate).toHaveBeenCalledWith('/reading')
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
})
