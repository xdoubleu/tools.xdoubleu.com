import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import CategoryManager from '@/components/recipes/CategoryManager'

const mutate = jest.fn().mockResolvedValue(undefined)
const createCategory = jest.fn().mockResolvedValue({})
const renameCategory = jest.fn().mockResolvedValue({})
const deleteCategory = jest.fn().mockResolvedValue({})

jest.mock('@/hooks/useShoppingList', () => ({
  useCategories: () => ({
    data: { categories: [{ id: 'cat-1', name: 'Produce' }] },
    isLoading: false,
    mutate
  })
}))

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ createCategory, renameCategory, deleteCategory })
}))

beforeEach(() => {
  jest.clearAllMocks()
})

describe('CategoryManager', () => {
  it('renders existing categories', () => {
    render(<CategoryManager />)
    expect(screen.getByText('Produce')).toBeInTheDocument()
  })

  it('creates a category', async () => {
    render(<CategoryManager />)
    fireEvent.change(screen.getByPlaceholderText(/New category/), {
      target: { value: 'Dairy' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(createCategory).toHaveBeenCalledWith({ name: 'Dairy' }))
    expect(mutate).toHaveBeenCalled()
  })

  it('renames a category', async () => {
    render(<CategoryManager />)
    fireEvent.click(screen.getByRole('button', { name: 'Rename' }))
    fireEvent.change(screen.getByDisplayValue('Produce'), { target: { value: 'Veg' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(renameCategory).toHaveBeenCalledWith({ id: 'cat-1', name: 'Veg' }))
  })

  it('deletes a category', async () => {
    render(<CategoryManager />)
    fireEvent.click(screen.getByRole('button', { name: /Delete Produce/ }))
    await waitFor(() => expect(deleteCategory).toHaveBeenCalledWith({ id: 'cat-1' }))
  })

  it('shows an error when a mutation fails', async () => {
    createCategory.mockRejectedValueOnce(new Error('conflict'))
    render(<CategoryManager />)
    fireEvent.change(screen.getByPlaceholderText(/New category/), {
      target: { value: 'Produce' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(screen.getByText(/Something went wrong/)).toBeInTheDocument())
  })
})
