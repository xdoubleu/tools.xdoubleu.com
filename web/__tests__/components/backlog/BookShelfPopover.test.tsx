import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { UserBookSchema, BookSchema } from '@/lib/gen/backlog/v1/books_pb'

const mockUpdateBookStatus = jest.fn()
const mockToggleTag = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

jest.mock('@/hooks/useBacklog', () => ({
  useUpdateBookStatus: () => mockUpdateBookStatus,
  useToggleTag: () => mockToggleTag
}))

import BookShelfPopover from '@/components/backlog/BookShelfPopover'

function makeBook(tags: string[] = [], status = 'to-read') {
  return create(UserBookSchema, {
    id: 'ub-1',
    bookId: 'book-1',
    status,
    rating: 3,
    tags,
    formats: [],
    book: create(BookSchema, { title: 'Dune', authors: ['Frank Herbert'] })
  })
}

function openPopover() {
  fireEvent.click(screen.getByLabelText('Edit shelves and tags'))
}

describe('BookShelfPopover', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockToggleTag.mockReset()
    mockMutate.mockReset()
    mockUpdateBookStatus.mockResolvedValue({})
    mockToggleTag.mockResolvedValue({})
  })

  it('renders a visible trigger button', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByLabelText('Edit shelves and tags')).toBeInTheDocument()
    expect(screen.getByText('Shelves & tags')).toBeInTheDocument()
  })

  it('shows tag count in trigger when display tags are present', () => {
    render(
      <BookShelfPopover
        userBook={makeBook(['fantasy', 'sci-fi'])}
        knownShelves={[]}
        knownTags={['fantasy', 'sci-fi']}
      />
    )
    expect(screen.getByText('Shelves & tags (2)')).toBeInTheDocument()
  })

  it('does not count special tags in the trigger count', () => {
    render(<BookShelfPopover userBook={makeBook(['favourite'])} knownShelves={[]} knownTags={[]} />)
    expect(screen.getByText('Shelves & tags')).toBeInTheDocument()
    expect(screen.queryByText(/\(\d+\)/)).not.toBeInTheDocument()
  })

  it('opens the panel on trigger click', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('closes the panel on second click (toggle)', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Edit shelves and tags'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes the panel on Escape', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} />)
    openPopover()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows a radio group for shelves inside the popover', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.getByRole('radiogroup')).toBeInTheDocument()
    expect(screen.getByLabelText('Want to read')).toBeInTheDocument()
    expect(screen.getByLabelText('Read')).toBeInTheDocument()
  })

  it('shows custom shelves in the radio group', () => {
    render(
      <BookShelfPopover
        userBook={makeBook()}
        knownShelves={['classics', 'sci-fi']}
        knownTags={[]}
      />
    )
    openPopover()
    expect(screen.getByLabelText('classics')).toBeInTheDocument()
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })

  it('shows known tags as checkboxes', () => {
    render(
      <BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={['fantasy', 'sci-fi']} />
    )
    openPopover()
    expect(screen.getByLabelText('fantasy')).toBeInTheDocument()
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })

  it('shows "No tags yet." when there are no known tags and book has no tags', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.getByText('No tags yet.')).toBeInTheDocument()
  })

  it('does not render an add combobox or button (select-only)', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} />)
    openPopover()
    expect(screen.queryByPlaceholderText(/add a shelf/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^add$/i })).not.toBeInTheDocument()
  })

  it('does not render remove badges for tags (select-only)', () => {
    render(
      <BookShelfPopover
        userBook={makeBook(['fantasy'])}
        knownShelves={[]}
        knownTags={['fantasy']}
      />
    )
    openPopover()
    expect(screen.queryByRole('button', { name: /remove/i })).not.toBeInTheDocument()
  })

  it('calls updateBookStatus when a shelf radio is selected', async () => {
    render(<BookShelfPopover userBook={makeBook([], 'to-read')} knownShelves={[]} knownTags={[]} />)
    openPopover()
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() =>
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({ bookId: 'book-1', status: 'read' })
      )
    )
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('calls toggleTag when a tag checkbox is checked', async () => {
    render(
      <BookShelfPopover
        userBook={makeBook([], 'to-read')}
        knownShelves={[]}
        knownTags={['fantasy']}
      />
    )
    openPopover()
    fireEvent.click(screen.getByLabelText('fantasy'))
    await waitFor(() => expect(mockToggleTag).toHaveBeenCalledWith('book-1', 'fantasy'))
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('calls onSaved after status change', async () => {
    const onSaved = jest.fn()
    render(
      <BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={[]} onSaved={onSaved} />
    )
    openPopover()
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })

  it('calls onSaved after tag toggle', async () => {
    const onSaved = jest.fn()
    render(
      <BookShelfPopover
        userBook={makeBook()}
        knownShelves={[]}
        knownTags={['mystery']}
        onSaved={onSaved}
      />
    )
    openPopover()
    fireEvent.click(screen.getByLabelText('mystery'))
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
  })

  it('shows error message when status update fails', async () => {
    mockUpdateBookStatus.mockRejectedValueOnce(new Error('network'))
    render(<BookShelfPopover userBook={makeBook([], 'to-read')} knownShelves={[]} knownTags={[]} />)
    openPopover()
    fireEvent.click(screen.getByLabelText('Read'))
    await waitFor(() => {
      expect(screen.getByText('Failed to update status.')).toBeInTheDocument()
    })
  })

  it('reverts tags optimistically on toggleTag failure', async () => {
    mockToggleTag.mockRejectedValueOnce(new Error('network'))
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} knownTags={['mystery']} />)
    openPopover()
    fireEvent.click(screen.getByLabelText('mystery'))
    await waitFor(() => {
      expect(screen.getByText('Failed to update tag.')).toBeInTheDocument()
    })
  })

  it('filters special tags from the tag list', () => {
    render(
      <BookShelfPopover
        userBook={makeBook(['favourite'])}
        knownShelves={[]}
        knownTags={['favourite', 'sci-fi']}
      />
    )
    openPopover()
    expect(screen.queryByLabelText('favourite')).not.toBeInTheDocument()
    expect(screen.getByLabelText('sci-fi')).toBeInTheDocument()
  })
})
