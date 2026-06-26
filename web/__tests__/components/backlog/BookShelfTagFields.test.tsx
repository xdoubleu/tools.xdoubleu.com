import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateBookStatus = jest.fn()
const mockToggleTag = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus,
  useToggleTag: () => mockToggleTag
}))

jest.mock('swr', () => ({
  mutate: jest.fn()
}))

import BookShelfTagFields from '@/components/backlog/BookShelfTagFields'
import { mutate as mockMutate } from 'swr'

const mockMutateFn = jest.mocked(mockMutate)

function makeUserBook(overrides = {}) {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status: 'to-read',
    rating: 0,
    tags: [],
    formats: [],
    book: create(BookSchema, { title: 'Test Book', authors: ['Test Author'] }),
    ...overrides
  })
}

describe('BookShelfTagFields', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUpdateBookStatus.mockResolvedValue(undefined)
    mockToggleTag.mockResolvedValue(undefined)
  })

  it('renders a radio group with built-in shelves', () => {
    render(<BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
    expect(screen.getByLabelText('Want to read')).toBeInTheDocument()
    expect(screen.getByLabelText('Currently reading')).toBeInTheDocument()
    expect(screen.getByLabelText('Read')).toBeInTheDocument()
    expect(screen.getByLabelText('Dropped')).toBeInTheDocument()
  })

  it('renders custom shelves in the radio group', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook()}
        knownShelves={['classics', 'sci-fi']}
        knownTags={[]}
      />
    )
    expect(screen.getByLabelText('classics')).toBeInTheDocument()
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })

  it('does not show built-in statuses as custom shelves', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook()}
        knownShelves={['to-read', 'sci-fi']}
        knownTags={[]}
      />
    )
    // 'to-read' is built-in — should appear exactly once (from BOOK_STATUSES), not twice
    expect(screen.getAllByLabelText('Want to read')).toHaveLength(1)
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })

  it('marks the current shelf as selected', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ status: 'read' })}
        knownShelves={[]}
        knownTags={[]}
      />
    )
    expect(screen.getByLabelText<HTMLInputElement>('Read').checked).toBe(true)
  })

  it('calls updateBookStatus when a radio is selected', async () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ status: 'to-read' })}
        knownShelves={[]}
        knownTags={[]}
      />
    )
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() =>
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({ bookId: 'book-1', status: 'read' })
      )
    )
    expect(mockMutateFn).toHaveBeenCalledWith('/backlog/books')
  })

  it('reverts status on updateBookStatus failure and shows error', async () => {
    mockUpdateBookStatus.mockRejectedValueOnce(new Error('network'))
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ status: 'to-read' })}
        knownShelves={[]}
        knownTags={[]}
      />
    )
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() => expect(screen.getByText('Failed to update status.')).toBeInTheDocument())
    // Should have reverted: 'Want to read' radio should be checked again
    expect(screen.getByLabelText<HTMLInputElement>('Want to read').checked).toBe(true)
  })

  it('renders known tags as checkboxes', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook()}
        knownShelves={[]}
        knownTags={['fantasy', 'mystery']}
      />
    )
    expect(screen.getByLabelText('fantasy')).toBeInTheDocument()
    expect(screen.getByLabelText('mystery')).toBeInTheDocument()
  })

  it('shows "No tags yet." when there are no known tags and book has none', () => {
    render(<BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('No tags yet.')).toBeInTheDocument()
  })

  it('checks the checkbox for tags already on the book', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['fantasy'] })}
        knownShelves={[]}
        knownTags={['fantasy', 'mystery']}
      />
    )
    expect(screen.getByLabelText<HTMLInputElement>('fantasy').checked).toBe(true)
    expect(screen.getByLabelText<HTMLInputElement>('mystery').checked).toBe(false)
  })

  it('calls toggleTag when a checkbox is checked', async () => {
    render(
      <BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={['fantasy']} />
    )
    fireEvent.click(screen.getByLabelText('fantasy'))
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'fantasy'))
    expect(mockMutateFn).toHaveBeenCalledWith('/backlog/books')
  })

  it('calls toggleTag when a checked tag is unchecked', async () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['fantasy'] })}
        knownShelves={[]}
        knownTags={['fantasy']}
      />
    )
    fireEvent.click(screen.getByLabelText('fantasy'))
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'fantasy'))
  })

  it('reverts tag on toggleTag failure and shows error', async () => {
    mockToggleTag.mockRejectedValueOnce(new Error('network'))
    render(
      <BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={['fantasy']} />
    )
    fireEvent.click(screen.getByLabelText('fantasy'))
    await waitFor(() => expect(screen.getByText('Failed to update tag.')).toBeInTheDocument())
    // Checkbox reverted to unchecked
    expect(screen.getByLabelText<HTMLInputElement>('fantasy').checked).toBe(false)
  })

  it('calls onSaved after a successful status change', async () => {
    const onSaved = jest.fn()
    render(
      <BookShelfTagFields
        userBook={makeUserBook()}
        knownShelves={[]}
        knownTags={[]}
        onSaved={onSaved}
      />
    )
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })

  it('calls onSaved after a successful tag toggle', async () => {
    const onSaved = jest.fn()
    render(
      <BookShelfTagFields
        userBook={makeUserBook()}
        knownShelves={[]}
        knownTags={['mystery']}
        onSaved={onSaved}
      />
    )
    fireEvent.click(screen.getByLabelText('mystery'))
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })

  it('filters special tags from the tag list', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['favourite'] })}
        knownShelves={[]}
        knownTags={['favourite', 'sci-fi']}
      />
    )
    expect(screen.queryByLabelText('favourite')).not.toBeInTheDocument()
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })

  it('does not render any add combobox or remove button (select-only)', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['fantasy'] })}
        knownShelves={['classics']}
        knownTags={['fantasy']}
      />
    )
    expect(screen.queryByRole('button', { name: /add|set|remove/i })).not.toBeInTheDocument()
    expect(screen.queryByPlaceholderText(/add|custom shelf/i)).not.toBeInTheDocument()
  })

  it('renders orphan tags (on book but not in knownTags) as checked checkboxes', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['legacy-tag'] })}
        knownShelves={[]}
        knownTags={[]}
      />
    )
    expect(screen.getByLabelText<HTMLInputElement>('legacy-tag').checked).toBe(true)
  })
})
