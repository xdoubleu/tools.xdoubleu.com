import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import AddManualBookModal from '@/components/books/AddManualBookModal'
import { useLibrary } from '@/hooks/useBooks'
import { create } from '@bufbuild/protobuf'
import {
  GetLibraryResponseSchema,
  LibraryResponseSchema,
  BookShelfSchema
} from '@/lib/gen/books/v1/library_pb'

const mockAddBook = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useCreateBook: () => mockAddBook,
  useLibrary: jest.fn()
}))

function makeLibraryData(shelfNames: string[] = []) {
  return create(GetLibraryResponseSchema, {
    library: create(LibraryResponseSchema, {
      shelves: shelfNames.map((name) => create(BookShelfSchema, { name }))
    })
  })
}

describe('AddManualBookModal', () => {
  beforeEach(() => {
    mockAddBook.mockReset()
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData(),
      isLoading: false,
      error: undefined
    })
  })

  it('defaults the title to empty when initialTitle is omitted', () => {
    render(<AddManualBookModal onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getByLabelText('Title')).toHaveValue('')
  })

  it('tolerates a missing library response when computing custom shelves', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })
    render(<AddManualBookModal initialTitle="A Title" onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getByLabelText('Status')).toHaveValue('to-read')
  })

  it('prefills the title from initialTitle', () => {
    render(
      <AddManualBookModal
        initialTitle="Dungeon Crawler Carl"
        onClose={jest.fn()}
        onAdded={jest.fn()}
      />
    )
    expect(screen.getByLabelText('Title')).toHaveValue('Dungeon Crawler Carl')
  })

  it('disables submit until a title is present', () => {
    render(<AddManualBookModal initialTitle="" onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getByRole('button', { name: 'Add Book' })).toBeDisabled()

    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'A Title' } })
    expect(screen.getByRole('button', { name: 'Add Book' })).toBeEnabled()
  })

  it('includes custom shelves from the library as selectable options', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useLibrary).mockReturnValue({
      data: makeLibraryData(['book-club']),
      isLoading: false,
      error: undefined
    })
    render(<AddManualBookModal initialTitle="A Title" onClose={jest.fn()} onAdded={jest.fn()} />)
    expect(screen.getByRole('option', { name: 'book-club' })).toBeInTheDocument()
  })

  it('calls onClose when Cancel is clicked', () => {
    const onClose = jest.fn()
    render(<AddManualBookModal initialTitle="A Title" onClose={onClose} onAdded={jest.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('submits with provider "manual", an empty providerId, and every typed field', async () => {
    const onAdded = jest.fn()
    const onClose = jest.fn()
    mockAddBook.mockResolvedValue(undefined)
    render(
      <AddManualBookModal initialTitle="Dungeon Crawler Carl" onClose={onClose} onAdded={onAdded} />
    )

    fireEvent.change(screen.getByLabelText('Author'), { target: { value: 'Matt Dinniman' } })
    fireEvent.change(screen.getByLabelText('ISBN-13'), { target: { value: '9780000000123' } })
    fireEvent.change(screen.getByLabelText('Cover URL'), {
      target: { value: 'https://example.com/cover.jpg' }
    })
    fireEvent.change(screen.getByLabelText('Description'), {
      target: { value: 'A litRPG dungeon crawl.' }
    })
    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'currently-reading' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))

    await waitFor(() => {
      expect(mockAddBook).toHaveBeenCalledWith(
        expect.objectContaining({
          provider: 'manual',
          providerId: '',
          title: 'Dungeon Crawler Carl',
          author: 'Matt Dinniman',
          isbn13: '9780000000123',
          coverUrl: 'https://example.com/cover.jpg',
          description: 'A litRPG dungeon crawl.',
          status: 'currently-reading'
        })
      )
      expect(onAdded).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('closes when Escape is pressed', () => {
    const onClose = jest.fn()
    render(<AddManualBookModal initialTitle="A Title" onClose={onClose} onAdded={jest.fn()} />)
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('shows an error message when addBook throws', async () => {
    mockAddBook.mockRejectedValue(new Error('Network error'))
    render(<AddManualBookModal initialTitle="A Title" onClose={jest.fn()} onAdded={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))

    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })

  it('shows a fallback error message when addBook throws a non-Error value', async () => {
    mockAddBook.mockRejectedValue('boom')
    render(<AddManualBookModal initialTitle="A Title" onClose={jest.fn()} onAdded={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add Book' }))

    await waitFor(() => {
      expect(screen.getByText('Failed to add book.')).toBeInTheDocument()
    })
  })

  it('does not submit when the title is blank/whitespace', () => {
    render(<AddManualBookModal initialTitle="   " onClose={jest.fn()} onAdded={jest.fn()} />)
    // The submit button is disabled for a blank title (so a click never
    // reaches the form), but handleSubmit guards against it too — submit
    // the form directly to exercise that guard. Dialog content renders in a
    // portal, so look the form up from a field inside it rather than the
    // render() container.
    fireEvent.submit(screen.getByLabelText('Title').closest('form')!)
    expect(mockAddBook).not.toHaveBeenCalled()
  })
})
