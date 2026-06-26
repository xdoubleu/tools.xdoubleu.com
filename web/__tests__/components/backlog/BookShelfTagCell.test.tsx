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

import BookShelfTagCell from '@/components/backlog/BookShelfTagCell'
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
    book: create(BookSchema, { title: 'Dune', authors: ['Frank Herbert'] }),
    ...overrides
  })
}

function openPopover() {
  fireEvent.click(screen.getByRole('button', { name: 'Edit shelf and tags' }))
}

describe('BookShelfTagCell', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUpdateBookStatus.mockResolvedValue(undefined)
    mockToggleTag.mockResolvedValue(undefined)
  })

  it('renders the current shelf label as the trigger', () => {
    const ub = makeUserBook({ status: 'to-read' })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('Want to read')).toBeInTheDocument()
  })

  it('renders custom shelf name as-is when not a built-in status', () => {
    const ub = makeUserBook({ status: 'sci-fi' })
    render(<BookShelfTagCell userBook={ub} knownShelves={['sci-fi']} knownTags={[]} />)
    expect(screen.getByText('sci-fi')).toBeInTheDocument()
  })

  it('shows a tag count badge when the book has display tags', () => {
    const ub = makeUserBook({ tags: ['fantasy'] })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={['fantasy']} />)
    expect(screen.getByText('+1')).toBeInTheDocument()
  })

  it('does not show tag count badge when there are no display tags', () => {
    const ub = makeUserBook({ tags: ['favourite'] }) // special tag — not displayed
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={[]} />)
    expect(screen.queryByText('+1')).not.toBeInTheDocument()
  })

  it('opens popover with radio group on trigger click', () => {
    const ub = makeUserBook()
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
    // Built-in statuses are rendered as radio items
    expect(screen.getByLabelText('Want to read')).toBeInTheDocument()
    expect(screen.getByLabelText('Read')).toBeInTheDocument()
  })

  it('shows custom shelves in the radio group', () => {
    const ub = makeUserBook({ status: 'to-read' })
    render(<BookShelfTagCell userBook={ub} knownShelves={['classics', 'sci-fi']} knownTags={[]} />)
    openPopover()
    expect(screen.getByLabelText('classics')).toBeInTheDocument()
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })

  it('shows known tags as checkboxes', () => {
    const ub = makeUserBook()
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={['fantasy', 'sci-fi']} />)
    openPopover()
    expect(screen.getByLabelText('fantasy')).toBeInTheDocument()
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })

  it('calls updateBookStatus when a radio item is selected', async () => {
    const ub = makeUserBook({ status: 'to-read', tags: [] })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={[]} />)
    openPopover()
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() =>
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({ bookId: 'book-1', status: 'read' })
      )
    )
    await waitFor(() => expect(mockMutateFn).toHaveBeenCalledWith('/backlog/books'))
  })

  it('calls toggleTag when a checkbox is ticked', async () => {
    const ub = makeUserBook({ tags: [] })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={['fantasy']} />)
    openPopover()
    fireEvent.click(screen.getByLabelText('fantasy'))
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'fantasy'))
    await waitFor(() => expect(mockMutateFn).toHaveBeenCalledWith('/backlog/books'))
  })

  it('shows "No tags yet" when there are no known tags', () => {
    const ub = makeUserBook({ tags: [] })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.getByText('No tags yet.')).toBeInTheDocument()
  })

  it('does not render remove badges for tags (select-only)', () => {
    const ub = makeUserBook({ tags: ['fantasy'] })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={['fantasy']} />)
    openPopover()
    expect(screen.queryByRole('button', { name: /Remove tag/i })).not.toBeInTheDocument()
  })

  it('does not render an add-tag combobox or Set button (select-only)', () => {
    const ub = makeUserBook()
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.queryByPlaceholderText(/custom shelf/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /set/i })).not.toBeInTheDocument()
    expect(screen.queryByPlaceholderText(/add tag/i)).not.toBeInTheDocument()
  })

  it('reverts status optimistically on updateBookStatus failure', async () => {
    mockUpdateBookStatus.mockRejectedValue(new Error('network'))
    const ub = makeUserBook({ status: 'to-read' })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={[]} />)
    openPopover()
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() => expect(screen.getByText('Failed to update status.')).toBeInTheDocument())
  })

  it('reverts tags optimistically on toggleTag failure', async () => {
    mockToggleTag.mockRejectedValue(new Error('network'))
    const ub = makeUserBook({ tags: [] })
    render(<BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={['fantasy']} />)
    openPopover()
    fireEvent.click(screen.getByLabelText('fantasy'))
    await waitFor(() => expect(screen.getByText('Failed to update tag.')).toBeInTheDocument())
  })

  it('filters special tags out of the tag list', () => {
    const ub = makeUserBook({ tags: ['favourite'] })
    render(
      <BookShelfTagCell userBook={ub} knownShelves={[]} knownTags={['favourite', 'fantasy']} />
    )
    openPopover()
    // 'favourite' is a special tag — should not appear as a checkbox
    expect(screen.queryByLabelText('favourite')).not.toBeInTheDocument()
    expect(screen.getByLabelText('fantasy')).toBeInTheDocument()
  })
})
