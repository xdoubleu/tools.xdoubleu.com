import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
import BookEditDialog from '@/components/books/BookEditDialog'
import { BookSchema, type Book } from '@/lib/gen/books/v1/library_pb'

const mockUpdateBook = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useUpdateBook: () => mockUpdateBook
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

function makeBook(overrides: MessageInitShape<typeof BookSchema> = {}): Book {
  return create(BookSchema, {
    id: 'book-1',
    title: 'Original Title',
    authors: ['Original Author'],
    isbn13: '9780140449112',
    coverUrl: 'https://example.com/cover.jpg',
    description: 'Original description.',
    pageCount: 200,
    ...overrides
  })
}

function renderDialog({
  book = makeBook(),
  open = true,
  onOpenChange = jest.fn(),
  onSaved = jest.fn()
}: {
  book?: Book
  open?: boolean
  onOpenChange?: (open: boolean) => void
  onSaved?: () => void
} = {}) {
  return render(
    <BookEditDialog book={book} open={open} onOpenChange={onOpenChange} onSaved={onSaved} />
  )
}

describe('BookEditDialog', () => {
  beforeEach(() => {
    mockUpdateBook.mockReset()
    mockMutate.mockReset()
  })

  it('renders the dialog pre-filled with the book fields when open', () => {
    renderDialog()
    expect(screen.getByText('Edit book')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Original Title')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Original Author')).toBeInTheDocument()
    expect(screen.getByDisplayValue('9780140449112')).toBeInTheDocument()
    expect(screen.getByDisplayValue('Original description.')).toBeInTheDocument()
    expect(screen.getByDisplayValue('200')).toBeInTheDocument()
    expect(screen.getByDisplayValue('https://example.com/cover.jpg')).toBeInTheDocument()
  })

  it('does not render content when closed', () => {
    renderDialog({ open: false })
    expect(screen.queryByText('Edit book')).not.toBeInTheDocument()
  })

  it('calls onOpenChange(false) on cancel', () => {
    const onOpenChange = jest.fn()
    renderDialog({ onOpenChange })
    fireEvent.click(screen.getByText('Cancel'))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('saves edited fields, splitting authors on comma', async () => {
    mockUpdateBook.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    const onOpenChange = jest.fn()
    const onSaved = jest.fn()
    renderDialog({ onOpenChange, onSaved })

    fireEvent.change(screen.getByDisplayValue('Original Title'), {
      target: { value: 'Edited Title' }
    })
    fireEvent.change(screen.getByDisplayValue('Original Author'), {
      target: { value: 'Author One, Author Two' }
    })

    fireEvent.click(screen.getByTestId('edit-book-save-btn'))

    await waitFor(() =>
      expect(mockUpdateBook).toHaveBeenCalledWith(
        'book-1',
        expect.objectContaining({
          title: 'Edited Title',
          authors: ['Author One', 'Author Two']
        })
      )
    )
    expect(mockMutate).toHaveBeenCalledWith('/books')
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(onSaved).toHaveBeenCalled()
  })

  it('saves edited isbn13, description, page count and cover URL', async () => {
    mockUpdateBook.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    renderDialog()

    fireEvent.change(screen.getByDisplayValue('9780140449112'), {
      target: { value: '9780140449113' }
    })
    fireEvent.change(screen.getByDisplayValue('Original description.'), {
      target: { value: 'Edited description.' }
    })
    fireEvent.change(screen.getByDisplayValue('200'), { target: { value: '321' } })
    fireEvent.change(screen.getByDisplayValue('https://example.com/cover.jpg'), {
      target: { value: 'https://example.com/new-cover.jpg' }
    })

    fireEvent.click(screen.getByTestId('edit-book-save-btn'))

    await waitFor(() =>
      expect(mockUpdateBook).toHaveBeenCalledWith(
        'book-1',
        expect.objectContaining({
          isbn13: '9780140449113',
          description: 'Edited description.',
          pageCount: 321,
          coverUrl: 'https://example.com/new-cover.jpg'
        })
      )
    )
  })

  it('clears the cover URL field when Remove is clicked', () => {
    renderDialog()
    fireEvent.click(screen.getByText('Remove'))
    expect(screen.getByPlaceholderText('No cover')).toHaveValue('')
  })

  it('sends an empty coverUrl after Remove + Save', async () => {
    mockUpdateBook.mockResolvedValue({})
    mockMutate.mockResolvedValue(undefined)
    renderDialog()

    fireEvent.click(screen.getByText('Remove'))
    fireEvent.click(screen.getByTestId('edit-book-save-btn'))

    await waitFor(() =>
      expect(mockUpdateBook).toHaveBeenCalledWith(
        'book-1',
        expect.objectContaining({ coverUrl: '' })
      )
    )
  })

  it('shows error message when save fails', async () => {
    mockUpdateBook.mockRejectedValue(new Error('network error'))
    renderDialog()

    fireEvent.click(screen.getByTestId('edit-book-save-btn'))

    await waitFor(() => expect(screen.getByTestId('edit-book-error')).toBeInTheDocument())
    expect(screen.getByTestId('edit-book-error')).toHaveTextContent('Failed to save changes')
  })
})
