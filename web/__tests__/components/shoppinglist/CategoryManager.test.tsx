import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import CategoryManager from '@/components/shoppinglist/CategoryManager'
import { enqueueWrite } from '@/lib/offline/outbox'
import {
  createCategoryWrite,
  deleteCategoryWrite,
  renameCategoryWrite
} from '@/lib/shoppinglist/offlineWrites'

jest.mock('@/hooks/useShoppingList', () => ({
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

jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: jest.fn().mockResolvedValue(undefined)
}))

const enqueue = jest.mocked(enqueueWrite)

beforeEach(() => {
  jest.clearAllMocks()
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

  it('queues a delete', async () => {
    render(<CategoryManager />)
    fireEvent.click(screen.getByRole('button', { name: /Delete Produce/ }))
    await waitFor(() => expect(enqueue).toHaveBeenCalledWith(deleteCategoryWrite, { id: 'cat-1' }))
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
