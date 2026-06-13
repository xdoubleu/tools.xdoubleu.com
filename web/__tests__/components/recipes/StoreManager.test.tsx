import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import StoreManager from '@/components/recipes/StoreManager'

// jsdom returns zeroed getBoundingClientRect, so @dnd-kit's pointer/keyboard
// sensors can't compute positions. Mock DndContext to expose onDragEnd via a
// test trigger button, which exercises the real handleDragEnd + arrayMove logic.
jest.mock('@dnd-kit/core', () => {
  const actual = jest.requireActual('@dnd-kit/core')
  return {
    ...actual,
    DndContext: ({
      children,
      onDragEnd
    }: {
      children: React.ReactNode
      onDragEnd: (event: { active: { id: string }; over: { id: string } }) => void
    }) => (
      <>
        <button
          aria-label="drag Dairy above Vegetables"
          onClick={() => onDragEnd({ active: { id: 'cat-dairy' }, over: { id: 'cat-veg' } })}
        />
        {children}
      </>
    )
  }
})

const mutateStores = jest.fn().mockResolvedValue(undefined)
const mutateStoreCategories = jest.fn().mockResolvedValue(undefined)
const createStore = jest.fn().mockResolvedValue({})
const deleteStore = jest.fn().mockResolvedValue({})
const setStoreCategories = jest.fn().mockResolvedValue({})

jest.mock('@/hooks/useShoppingList', () => ({
  useStores: () => ({
    data: { stores: [{ id: 'store-1', name: 'Colruyt' }] },
    isLoading: false,
    mutate: mutateStores
  }),
  useStoreCategories: () => ({
    data: {
      categories: [
        { id: 'cat-veg', name: 'Vegetables' },
        { id: 'cat-dairy', name: 'Dairy' }
      ]
    },
    mutate: mutateStoreCategories
  }),
  useCategories: () => ({
    data: {
      categories: [
        { id: 'cat-veg', name: 'Vegetables' },
        { id: 'cat-dairy', name: 'Dairy' },
        { id: 'cat-bakery', name: 'Bakery' }
      ]
    }
  })
}))

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ createStore, deleteStore, setStoreCategories })
}))

beforeEach(() => {
  jest.clearAllMocks()
})

describe('StoreManager', () => {
  it('renders stores', () => {
    render(<StoreManager />)
    expect(screen.getByText('Colruyt')).toBeInTheDocument()
  })

  it('creates a store', async () => {
    render(<StoreManager />)
    fireEvent.change(screen.getByPlaceholderText(/New store/), { target: { value: 'Aldi' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(createStore).toHaveBeenCalledWith({ name: 'Aldi' }))
  })

  it('deletes a store', async () => {
    render(<StoreManager />)
    fireEvent.click(screen.getByRole('button', { name: /Delete Colruyt/ }))
    await waitFor(() => expect(deleteStore).toHaveBeenCalledWith({ id: 'store-1' }))
  })

  it('shows the aisle order editor when editing a store', () => {
    render(<StoreManager />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit order' }))
    expect(screen.getByText('Aisle order')).toBeInTheDocument()
    expect(screen.getByText('Vegetables')).toBeInTheDocument()
    expect(screen.getByText('Dairy')).toBeInTheDocument()
  })

  it('reorders categories on drag and saves the new order', async () => {
    render(<StoreManager />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit order' }))
    // Drag Dairy above Vegetables so the order becomes Dairy, Vegetables.
    fireEvent.click(screen.getByRole('button', { name: 'drag Dairy above Vegetables' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save order' }))
    await waitFor(() =>
      expect(setStoreCategories).toHaveBeenCalledWith({
        storeId: 'store-1',
        categoryIds: ['cat-dairy', 'cat-veg']
      })
    )
  })

  it('exposes a reorder drag handle for each category', () => {
    render(<StoreManager />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit order' }))
    expect(screen.getByRole('button', { name: 'Reorder Vegetables' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reorder Dairy' })).toBeInTheDocument()
  })

  it('adds an available category to the order', async () => {
    render(<StoreManager />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit order' }))
    fireEvent.change(screen.getByLabelText('Add category to store'), {
      target: { value: 'cat-bakery' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save order' }))
    await waitFor(() =>
      expect(setStoreCategories).toHaveBeenCalledWith({
        storeId: 'store-1',
        categoryIds: ['cat-veg', 'cat-dairy', 'cat-bakery']
      })
    )
  })
})
