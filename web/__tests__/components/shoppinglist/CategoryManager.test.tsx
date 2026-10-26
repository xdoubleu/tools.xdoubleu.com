import React from 'react'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import CategoryManager from '@/components/shoppinglist/CategoryManager'
import { enqueueWrite } from '@/lib/offline/outbox'
import {
  createCategoryWrite,
  deleteCategoryWrite,
  renameCategoryWrite
} from '@/lib/shoppinglist/offlineWrites'

vi.mock('@/hooks/useIngredientCatalog', () => ({
  useCategories: () => ({
    data: {
      categories: [
        { id: 'cat-1', name: 'Produce' },
        { id: 'cat-2', name: 'Dairy' }
      ]
    },
    isLoading: false
  })
}))

vi.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: vi.fn().mockResolvedValue(undefined)
}))

const enqueue = vi.mocked(enqueueWrite)

beforeEach(() => {
  vi.clearAllMocks()
})

describe('CategoryManager', () => {
  it('renders existing categories', () => {
    render(<CategoryManager />)
    expect(screen.getByText('Produce')).toBeInTheDocument()
  })

  it('queues a category create with a client id', async () => {
    render(<CategoryManager />)
    fireEvent.change(screen.getByPlaceholderText(/New category/), {
      target: { value: 'Bakery' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(createCategoryWrite, {
        id: expect.stringMatching(/^[0-9a-f-]{36}$/),
        name: 'Bakery'
      })
    )
    expect(screen.getByPlaceholderText(/New category/)).toHaveValue('')
  })

  it('queues a rename', async () => {
    render(<CategoryManager />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Rename' })[0])
    fireEvent.change(screen.getByDisplayValue('Produce'), { target: { value: 'Veg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(renameCategoryWrite, { id: 'cat-1', name: 'Veg' })
    )
  })

  it('queues a delete after confirming', async () => {
    render(<CategoryManager />)
    fireEvent.click(screen.getByRole('button', { name: /Delete Produce/ }))
    const dialog = screen.getByRole('dialog', { name: 'Delete Produce?' })
    expect(dialog).toHaveTextContent(/unassigned/)
    expect(enqueue).not.toHaveBeenCalled()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(enqueue).toHaveBeenCalledWith(deleteCategoryWrite, { id: 'cat-1' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('keeps the category when the delete is cancelled', () => {
    render(<CategoryManager />)
    fireEvent.click(screen.getByRole('button', { name: /Delete Produce/ }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(enqueue).not.toHaveBeenCalled()
  })

  it('ignores blank names', () => {
    render(<CategoryManager />)
    fireEvent.change(screen.getByPlaceholderText(/New category/), { target: { value: '  ' } })
    fireEvent.submit(screen.getByPlaceholderText(/New category/))
    fireEvent.click(screen.getAllByRole('button', { name: 'Rename' })[0])
    fireEvent.change(screen.getByDisplayValue('Produce'), { target: { value: ' ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(enqueue).not.toHaveBeenCalled()
  })

  it('rejects a name already in use, ignoring case', () => {
    render(<CategoryManager />)
    fireEvent.change(screen.getByPlaceholderText(/New category/), {
      target: { value: 'produce' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(screen.getByText('That name is already in use.')).toBeInTheDocument()
    expect(enqueue).not.toHaveBeenCalled()
  })

  it('rejects renaming onto another category’s name', () => {
    render(<CategoryManager />)
    fireEvent.click(screen.getAllByRole('button', { name: 'Rename' })[0])
    fireEvent.change(screen.getByDisplayValue('Produce'), { target: { value: 'DAIRY' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('That name is already in use.')).toBeInTheDocument()
    expect(enqueue).not.toHaveBeenCalled()
  })
})
