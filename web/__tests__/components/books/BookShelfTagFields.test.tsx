import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'

const mockUpdateBookStatus = jest.fn()
const mockToggleTag = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus,
  useToggleTag: () => mockToggleTag
}))

jest.mock('swr', () => ({
  mutate: jest.fn()
}))

import BookShelfTagFields from '@/components/books/BookShelfTagFields'
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
    expect(mockMutateFn).toHaveBeenCalledWith('/books')
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

  it('renders known tags as clickable chips', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook()}
        knownShelves={[]}
        knownTags={['fantasy', 'mystery']}
      />
    )
    expect(screen.getByRole('button', { name: 'fantasy' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'mystery' })).toBeInTheDocument()
  })

  it('shows "No tags yet." when there are no known tags and book has none', () => {
    render(<BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('No tags yet.')).toBeInTheDocument()
  })

  it('marks chips for tags already on the book as active', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['fantasy'] })}
        knownShelves={[]}
        knownTags={['fantasy', 'mystery']}
      />
    )
    expect(screen.getByRole('button', { name: 'fantasy' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'mystery' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('calls toggleTag with a single click when an inactive chip is clicked', async () => {
    render(
      <BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={['fantasy']} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'fantasy' }))
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'fantasy'))
    expect(mockMutateFn).toHaveBeenCalledWith('/books')
  })

  it('calls toggleTag when an active chip is clicked (removes it)', async () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['fantasy'] })}
        knownShelves={[]}
        knownTags={['fantasy']}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'fantasy' }))
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'fantasy'))
  })

  it('reverts chip state on toggleTag failure and shows error', async () => {
    mockToggleTag.mockRejectedValueOnce(new Error('network'))
    render(
      <BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={['fantasy']} />
    )
    fireEvent.click(screen.getByRole('button', { name: 'fantasy' }))
    await waitFor(() => expect(screen.getByText('Failed to update tag.')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'fantasy' })).toHaveAttribute('aria-pressed', 'false')
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
    fireEvent.click(screen.getByRole('button', { name: 'mystery' }))
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
    expect(screen.queryByRole('button', { name: 'favourite' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'sci-fi' })).toBeInTheDocument()
  })

  it('renders orphan tags (on book but not in knownTags) as active chips', () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['legacy-tag'] })}
        knownShelves={[]}
        knownTags={[]}
      />
    )
    expect(screen.getByRole('button', { name: 'legacy-tag' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })

  it('adds a new tag via the combobox on Enter', async () => {
    render(
      <BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={['fantasy']} />
    )
    const combobox = screen.getByLabelText('Add a tag')
    fireEvent.change(combobox, { target: { value: 'fantasy' } })
    fireEvent.keyDown(combobox, { key: 'Enter' })
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'fantasy'))
  })

  it('adds a new tag via clicking a combobox suggestion', async () => {
    render(
      <BookShelfTagFields userBook={makeUserBook()} knownShelves={[]} knownTags={['mystery']} />
    )
    const combobox = screen.getByLabelText('Add a tag')
    fireEvent.change(combobox, { target: { value: 'mys' } })
    fireEvent.mouseDown(screen.getByText('mystery', { selector: 'li' }))
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'mystery'))
  })

  it('does not re-add a tag already on the book via the combobox', async () => {
    render(
      <BookShelfTagFields
        userBook={makeUserBook({ tags: ['fantasy'] })}
        knownShelves={[]}
        knownTags={['fantasy']}
      />
    )
    // fantasy is already active, so it must not appear in the addable suggestions
    const combobox = screen.getByLabelText('Add a tag')
    fireEvent.change(combobox, { target: { value: 'fantasy' } })
    expect(screen.queryByText('fantasy', { selector: 'li' })).not.toBeInTheDocument()
  })
})
