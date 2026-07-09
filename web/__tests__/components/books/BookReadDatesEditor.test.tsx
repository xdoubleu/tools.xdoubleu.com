import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

const mockUpdateFinishedAt = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useUpdateFinishedAt: () => mockUpdateFinishedAt
}))

jest.mock('swr', () => ({
  mutate: jest.fn()
}))

import BookReadDatesEditor from '@/components/books/BookReadDatesEditor'
import { mutate as mockMutate } from 'swr'

const mockMutateFn = jest.mocked(mockMutate)

function makeUserBook(overrides = {}) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status: 'read',
    tags: [],
    formats: [],
    finishedAt: [],
    book: create(BookSchema, { title: 'Test Book', authors: ['Test Author'] }),
    ...overrides
  })
}

describe('BookReadDatesEditor', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUpdateFinishedAt.mockResolvedValue(undefined)
  })

  it('renders one date input per existing finished date', () => {
    render(
      <BookReadDatesEditor
        userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z', '2024-06-01T00:00:00Z'] })}
      />
    )
    const inputs = screen.getAllByDisplayValue(/2024-/) as HTMLInputElement[]
    expect(inputs).toHaveLength(2)
    expect(inputs[0].value).toBe('2024-01-15')
    expect(inputs[1].value).toBe('2024-06-01')
  })

  it('labels a single date "Finished"', () => {
    render(
      <BookReadDatesEditor userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z'] })} />
    )
    expect(screen.getByText('Finished')).toBeInTheDocument()
  })

  it('labels multiple dates "Read dates"', () => {
    render(
      <BookReadDatesEditor
        userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z', '2024-06-01T00:00:00Z'] })}
      />
    )
    expect(screen.getByText('Read dates')).toBeInTheDocument()
  })

  it('adds a new empty date row when "Add date" is clicked', () => {
    const { container } = render(
      <BookReadDatesEditor userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z'] })} />
    )
    expect(container.querySelectorAll('input[type="date"]')).toHaveLength(1)
    fireEvent.click(screen.getByRole('button', { name: 'Add date' }))
    expect(container.querySelectorAll('input[type="date"]')).toHaveLength(2)
  })

  it('removes a date and saves when the remove button is clicked', async () => {
    render(
      <BookReadDatesEditor
        userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z', '2024-06-01T00:00:00Z'] })}
      />
    )
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove this date' })[0])
    await waitFor(() => expect(mockUpdateFinishedAt).toHaveBeenCalledWith('book-1', ['2024-06-01']))
    expect(mockMutateFn).toHaveBeenCalledWith('/books')
  })

  it('saves the edited date on blur', async () => {
    render(
      <BookReadDatesEditor userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z'] })} />
    )
    const input = screen.getByDisplayValue('2024-01-15')
    fireEvent.change(input, { target: { value: '2024-02-20' } })
    fireEvent.blur(input)
    await waitFor(() => expect(mockUpdateFinishedAt).toHaveBeenCalledWith('book-1', ['2024-02-20']))
  })

  it('reverts and shows an error when saving fails', async () => {
    mockUpdateFinishedAt.mockRejectedValueOnce(new Error('network'))
    render(
      <BookReadDatesEditor
        userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z', '2024-06-01T00:00:00Z'] })}
      />
    )
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove this date' })[0])
    await waitFor(() =>
      expect(screen.getByText('Failed to update read dates.')).toBeInTheDocument()
    )
    expect(screen.getAllByDisplayValue(/2024-/)).toHaveLength(2)
  })

  it('calls onSaved after a successful save', async () => {
    const onSaved = jest.fn()
    render(
      <BookReadDatesEditor
        userBook={makeUserBook({ finishedAt: ['2024-01-15T00:00:00Z'] })}
        onSaved={onSaved}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Remove this date' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })
})
