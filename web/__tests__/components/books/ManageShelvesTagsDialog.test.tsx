import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockRenameShelf = jest.fn()
const mockDeleteShelf = jest.fn()
const mockRenameTag = jest.fn()
const mockDeleteTag = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useRenameShelf: () => mockRenameShelf,
  useDeleteShelf: () => mockDeleteShelf,
  useRenameTag: () => mockRenameTag,
  useDeleteTag: () => mockDeleteTag
}))

jest.mock('swr', () => ({ mutate: jest.fn() }))

// LibrarySidebar is imported for its types — no DOM output needed
jest.mock('@/components/books/LibrarySidebar', () => ({
  buildShelves: jest.fn(),
  buildTags: jest.fn()
}))

import ManageShelvesTagsDialog from '@/components/books/ManageShelvesTagsDialog'
import type { Shelf } from '@/components/books/LibrarySidebar'
import { mutate as swrMutate } from 'swr'

const mockMutate = jest.mocked(swrMutate)

const builtInShelves: Shelf[] = [
  { id: 'currently-reading', label: 'Currently reading', count: 2 },
  { id: 'to-read', label: 'Want to read', count: 5 },
  { id: 'read', label: 'Read', count: 10 },
  { id: 'dropped', label: 'Dropped', count: 1 }
]

const customShelves: Shelf[] = [{ id: 'classics', label: 'classics', count: 3 }]

const allShelves = [...builtInShelves, ...customShelves]

function renderDialog(tags: string[] = []) {
  return render(
    <ManageShelvesTagsDialog
      open={true}
      onOpenChange={jest.fn()}
      shelves={allShelves}
      tags={tags}
    />
  )
}

describe('ManageShelvesTagsDialog', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockRenameShelf.mockResolvedValue(undefined)
    mockDeleteShelf.mockResolvedValue(undefined)
    mockRenameTag.mockResolvedValue(undefined)
    mockDeleteTag.mockResolvedValue(undefined)
  })

  it('renders the dialog with built-in and custom shelves', () => {
    renderDialog()
    expect(screen.getByText('Edit shelves & tags')).toBeInTheDocument()
    // Built-in shelves section
    expect(screen.getByText('Built-in shelves')).toBeInTheDocument()
    expect(screen.getByText('Currently reading')).toBeInTheDocument()
    expect(screen.getByText('Want to read')).toBeInTheDocument()
    // Custom shelf
    expect(screen.getByText('Custom shelves')).toBeInTheDocument()
    expect(screen.getByText('classics')).toBeInTheDocument()
  })

  it('shows "No custom shelves yet" when there are none', () => {
    render(
      <ManageShelvesTagsDialog
        open={true}
        onOpenChange={jest.fn()}
        shelves={builtInShelves}
        tags={[]}
      />
    )
    expect(screen.getByText('No custom shelves yet.')).toBeInTheDocument()
  })

  it('shows rename input when Rename is clicked for a custom shelf', () => {
    renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
    expect(screen.getByDisplayValue('classics')).toBeInTheDocument()
  })

  it('calls renameShelf and mutates on save', async () => {
    renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
    const input = screen.getByDisplayValue('classics')
    fireEvent.change(input, { target: { value: 'classic-lit' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(mockRenameShelf).toHaveBeenCalledWith('classics', 'classic-lit'))
    await waitFor(() => expect(mockMutate).toHaveBeenCalledWith('/books'))
  })

  it('cancels rename when Cancel is clicked', () => {
    renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
    expect(screen.getByDisplayValue('classics')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByDisplayValue('classics')).not.toBeInTheDocument()
  })

  it('shows delete confirm UI when Delete is clicked for a custom shelf', () => {
    renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(screen.getByText(/Move.*books from/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete & move' })).toBeInTheDocument()
  })

  it('calls deleteShelf with target on confirm', async () => {
    renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'Delete & move' }))
    await waitFor(() =>
      expect(mockDeleteShelf).toHaveBeenCalledWith('classics', expect.any(String))
    )
    await waitFor(() => expect(mockMutate).toHaveBeenCalledWith('/books'))
  })

  it('cancels delete when Cancel is clicked', () => {
    renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    const cancels = screen.getAllByRole('button', { name: 'Cancel' })
    fireEvent.click(cancels[cancels.length - 1])
    expect(screen.queryByRole('button', { name: 'Delete & move' })).not.toBeInTheDocument()
  })

  it('renders tags with rename/delete buttons', () => {
    renderDialog(['fantasy', 'sci-fi'])
    expect(screen.getByText('fantasy')).toBeInTheDocument()
    expect(screen.getByText('sci-fi')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Rename' }).length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByRole('button', { name: 'Delete' }).length).toBeGreaterThanOrEqual(1)
  })

  it('shows "No tags yet" when there are no tags', () => {
    renderDialog([])
    expect(screen.getByText('No tags yet.')).toBeInTheDocument()
  })

  it('calls renameTag and mutates on save', async () => {
    renderDialog(['fantasy'])
    // The first Rename button is for the custom shelf; the second is for the tag
    const renameButtons = screen.getAllByRole('button', { name: 'Rename' })
    fireEvent.click(renameButtons[renameButtons.length - 1])
    const input = screen.getByDisplayValue('fantasy')
    fireEvent.change(input, { target: { value: 'fantasy-lit' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(mockRenameTag).toHaveBeenCalledWith('fantasy', 'fantasy-lit'))
    await waitFor(() => expect(mockMutate).toHaveBeenCalledWith('/books'))
  })

  it('calls deleteTag and mutates when tag Delete is clicked', async () => {
    renderDialog(['fantasy'])
    const deleteButtons = screen.getAllByRole('button', { name: 'Delete' })
    fireEvent.click(deleteButtons[deleteButtons.length - 1])
    await waitFor(() => expect(mockDeleteTag).toHaveBeenCalledWith('fantasy'))
    await waitFor(() => expect(mockMutate).toHaveBeenCalledWith('/books'))
  })

  it('shows an error when renameShelf fails', async () => {
    mockRenameShelf.mockRejectedValue(new Error('Server error'))
    renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(screen.getByText('Server error')).toBeInTheDocument())
  })
})
