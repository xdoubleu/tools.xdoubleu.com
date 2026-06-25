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
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} />)
    expect(screen.getByLabelText('Edit shelves and tags')).toBeInTheDocument()
    expect(screen.getByText('Shelves & tags')).toBeInTheDocument()
  })

  it('shows tag count in trigger when tags are present', () => {
    render(<BookShelfPopover userBook={makeBook(['fantasy', 'sci-fi'])} knownShelves={[]} />)
    expect(screen.getByText('Shelves & tags (2)')).toBeInTheDocument()
  })

  it('opens the panel on trigger click', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} />)
    openPopover()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('closes the panel on second click (toggle)', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} />)
    openPopover()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Edit shelves and tags'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes the panel on Escape', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} />)
    openPopover()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows the status select inside the popover', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} />)
    openPopover()
    expect(screen.getByLabelText('Status')).toBeInTheDocument()
  })

  it('calls UpdateBookStatus when status changes', async () => {
    render(<BookShelfPopover userBook={makeBook([], 'to-read')} knownShelves={[]} />)
    openPopover()

    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'read' } })

    await waitFor(() => {
      expect(mockUpdateBookStatus).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'read', bookId: 'ub-1' })
      )
    })
    expect(mockMutate).toHaveBeenCalledWith('/backlog/books')
  })

  it('adds a tag via the Add button', async () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={['sci-fi']} />)
    openPopover()

    const combobox = screen.getByPlaceholderText('Add a shelf or tag...')
    fireEvent.change(combobox, { target: { value: 'fantasy' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'fantasy')
    })
  })

  it('removes a tag via the × button', async () => {
    render(<BookShelfPopover userBook={makeBook(['sci-fi'])} knownShelves={['sci-fi']} />)
    openPopover()

    fireEvent.click(screen.getByLabelText('Remove sci-fi'))

    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'sci-fi')
    })
  })

  it('shows existing tags as badges inside the popover', () => {
    render(<BookShelfPopover userBook={makeBook(['fantasy'])} knownShelves={[]} />)
    openPopover()
    expect(screen.getByText('fantasy')).toBeInTheDocument()
  })

  it('does not add a duplicate tag', async () => {
    render(<BookShelfPopover userBook={makeBook(['fantasy'])} knownShelves={[]} />)
    openPopover()

    const combobox = screen.getByPlaceholderText('Add a shelf or tag...')
    fireEvent.change(combobox, { target: { value: 'fantasy' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(mockToggleTag).not.toHaveBeenCalled()
    })
  })

  it('calls onSaved after status change', async () => {
    const onSaved = jest.fn()
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} onSaved={onSaved} />)
    openPopover()

    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'read' } })

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('calls onSaved after adding a tag', async () => {
    const onSaved = jest.fn()
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} onSaved={onSaved} />)
    openPopover()

    const combobox = screen.getByPlaceholderText('Add a shelf or tag...')
    fireEvent.change(combobox, { target: { value: 'new-tag' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(onSaved).toHaveBeenCalled()
    })
  })

  it('panel label reads "Shelves & tags"', () => {
    render(<BookShelfPopover userBook={makeBook(['fantasy'])} knownShelves={[]} />)
    openPopover()
    // The in-panel label (not the trigger) also says "Shelves & tags"
    const labels = screen.getAllByText('Shelves & tags')
    // trigger + panel label — at least one is in the dialog
    expect(labels.length).toBeGreaterThanOrEqual(1)
  })

  it('shows error message when status update fails', async () => {
    mockUpdateBookStatus.mockRejectedValueOnce(new Error('network'))
    render(<BookShelfPopover userBook={makeBook([], 'to-read')} knownShelves={[]} />)
    openPopover()

    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'read' } })

    await waitFor(() => {
      expect(screen.getByText('Failed to update status.')).toBeInTheDocument()
    })
  })

  it('rolls back and shows error when adding a tag fails', async () => {
    mockToggleTag.mockRejectedValueOnce(new Error('network'))
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} />)
    openPopover()

    const combobox = screen.getByPlaceholderText('Add a shelf or tag...')
    fireEvent.change(combobox, { target: { value: 'mystery' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(screen.getByText('Failed to add tag.')).toBeInTheDocument()
    })
    // Tag should be rolled back — badge gone
    expect(screen.queryByLabelText('Remove mystery')).not.toBeInTheDocument()
  })

  it('rolls back and shows error when removing a tag fails', async () => {
    mockToggleTag.mockRejectedValueOnce(new Error('network'))
    render(<BookShelfPopover userBook={makeBook(['sci-fi'])} knownShelves={[]} />)
    openPopover()

    fireEvent.click(screen.getByLabelText('Remove sci-fi'))

    await waitFor(() => {
      expect(screen.getByText('Failed to remove tag.')).toBeInTheDocument()
    })
    // Tag should be restored
    expect(screen.getByLabelText('Remove sci-fi')).toBeInTheDocument()
  })
})
