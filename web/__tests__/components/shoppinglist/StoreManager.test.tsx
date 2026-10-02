import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import StoreManager from '@/components/shoppinglist/StoreManager'
import { enqueueWrite } from '@/lib/offline/outbox'
import {
  createStoreWrite,
  deleteStoreWrite,
  setStoreCategoriesWrite
} from '@/lib/shoppinglist/offlineWrites'

// jsdom has no layout, so @dnd-kit can't drag; a trigger button calls
// onDragEnd to exercise the real reorder logic.
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

jest.mock('@/hooks/useShoppingList', () => ({
  useStores: () => ({
    data: { stores: [{ id: 'store-1', name: 'Colruyt' }] },
    isLoading: false
  }),
  useStoreCategories: () => ({
    data: {
      categories: [
        { id: 'cat-veg', name: 'Vegetables' },
        { id: 'cat-dairy', name: 'Dairy' }
      ]
    }
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

jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: jest.fn().mockResolvedValue(undefined)
}))

const enqueue = jest.mocked(enqueueWrite)

beforeEach(() => {
  jest.clearAllMocks()
})

describe('StoreManager', () => {
  it('renders stores', () => {
    render(<StoreManager />)
    expect(screen.getByText('Colruyt')).toBeInTheDocument()
  })

  it('queues a store create with a client id', async () => {
    render(<StoreManager />)
    fireEvent.change(screen.getByPlaceholderText(/New store/), { target: { value: 'Aldi' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(createStoreWrite, {
        id: expect.stringMatching(/^[0-9a-f-]{36}$/),
        name: 'Aldi'
      })
    )
  })

  it('ignores a blank store name', () => {
    render(<StoreManager />)
    fireEvent.change(screen.getByPlaceholderText(/New store/), { target: { value: ' ' } })
    fireEvent.submit(screen.getByPlaceholderText(/New store/))
    expect(enqueue).not.toHaveBeenCalled()
  })

  it('queues deleting a store whose editor is closed', async () => {
    render(<StoreManager />)
    fireEvent.click(screen.getByRole('button', { name: /Delete Colruyt/ }))
    await waitFor(() => expect(enqueue).toHaveBeenCalledWith(deleteStoreWrite, { id: 'store-1' }))
  })

  it('rejects a store name already in use', () => {
    render(<StoreManager />)
    fireEvent.change(screen.getByPlaceholderText(/New store/), { target: { value: 'colruyt' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(screen.getByText('That name is already in use.')).toBeInTheDocument()
    expect(enqueue).not.toHaveBeenCalled()
  })

  it('queues a store delete and closes its editor', async () => {
    render(<StoreManager />)
    fireEvent.click(screen.getByRole('button', { name: 'Edit order' }))
    fireEvent.click(screen.getByRole('button', { name: /Delete Colruyt/ }))
    await waitFor(() => expect(enqueue).toHaveBeenCalledWith(deleteStoreWrite, { id: 'store-1' }))
    expect(screen.queryByText('Aisle order')).not.toBeInTheDocument()
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
    fireEvent.click(screen.getByRole('button', { name: 'drag Dairy above Vegetables' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save order' }))
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(
        setStoreCategoriesWrite,
        { storeId: 'store-1', categoryIds: ['cat-dairy', 'cat-veg'] },
        { 'cat-dairy': 'Dairy', 'cat-veg': 'Vegetables' }
      )
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
      expect(enqueue).toHaveBeenCalledWith(
        setStoreCategoriesWrite,
        { storeId: 'store-1', categoryIds: ['cat-veg', 'cat-dairy', 'cat-bakery'] },
        { 'cat-veg': 'Vegetables', 'cat-dairy': 'Dairy', 'cat-bakery': 'Bakery' }
      )
    )
  })
})
