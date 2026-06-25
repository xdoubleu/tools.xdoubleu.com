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
    notes: 'n',
    book: create(BookSchema, { title: 'Dune', authors: ['Frank Herbert'] })
  })
}

function openPopover() {
  fireEvent.click(screen.getByLabelText('Edit status and shelves'))
}

describe('BookShelfPopover', () => {
  beforeEach(() => {
    mockUpdateBookStatus.mockReset()
    mockToggleTag.mockReset()
    mockMutate.mockReset()
    mockUpdateBookStatus.mockResolvedValue({})
    mockToggleTag.mockResolvedValue({})
  })

  it('renders a trigger button', () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={[]} />)
    expect(screen.getByLabelText('Edit status and shelves')).toBeInTheDocument()
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
    fireEvent.click(screen.getByLabelText('Edit status and shelves'))
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

  it('adds a shelf via the Add button', async () => {
    render(<BookShelfPopover userBook={makeBook()} knownShelves={['sci-fi']} />)
    openPopover()

    const combobox = screen.getByPlaceholderText('Add a shelf...')
    fireEvent.change(combobox, { target: { value: 'fantasy' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'fantasy')
    })
  })

  it('removes a shelf via the × button', async () => {
    render(<BookShelfPopover userBook={makeBook(['sci-fi'])} knownShelves={['sci-fi']} />)
    openPopover()

    fireEvent.click(screen.getByLabelText('Remove shelf sci-fi'))

    await waitFor(() => {
      expect(mockToggleTag).toHaveBeenCalledWith('ub-1', 'sci-fi')
    })
  })

  it('shows existing shelves as badges inside the popover', () => {
    render(<BookShelfPopover userBook={makeBook(['fantasy'])} knownShelves={[]} />)
    openPopover()
    expect(screen.getByText('fantasy')).toBeInTheDocument()
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
})
