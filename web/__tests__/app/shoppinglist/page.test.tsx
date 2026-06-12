import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const addShoppingItem = jest.fn().mockResolvedValue({ item: { id: 'i1' } })
const setItemCategory = jest.fn().mockResolvedValue({})
const createCategory = jest.fn().mockResolvedValue({ category: { id: 'cat-new' } })
const deleteShoppingItem = jest.fn().mockResolvedValue({})
const listMutate = jest.fn().mockResolvedValue(undefined)
const categoriesMutate = jest.fn().mockResolvedValue(undefined)

jest.mock('@/hooks/useShoppingList', () => ({
  useCustomList: () => ({ data: { items: [] }, isLoading: false, mutate: listMutate }),
  useCategories: () => ({
    data: { categories: [{ id: 'cat-produce', name: 'Produce' }] },
    mutate: categoriesMutate
  }),
  useAccessibleLists: () => ({ data: { owners: [] } }),
  useShoppingListShares: () => ({ data: { shares: [] }, mutate: jest.fn() }),
  useShareShoppingList: () => jest.fn().mockResolvedValue(undefined),
  useUnshareShoppingList: () => jest.fn().mockResolvedValue(undefined)
}))

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({
    addShoppingItem,
    setItemCategory,
    createCategory,
    deleteShoppingItem
  })
}))

import ShoppingPage from '@/app/shoppinglist/page'

beforeEach(() => jest.clearAllMocks())

describe('ShoppingPage add form', () => {
  it('assigns the chosen category to the catalog on add', async () => {
    render(<ShoppingPage />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Apples' } })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'cat-produce' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(addShoppingItem).toHaveBeenCalledWith({
        name: 'Apples',
        amount: '0',
        unit: '',
        ownerUserId: ''
      })
    )
    expect(setItemCategory).toHaveBeenCalledWith({
      name: 'Apples',
      categoryId: 'cat-produce',
      ownerUserId: ''
    })
  })

  it('skips the catalog write when no category is chosen', async () => {
    render(<ShoppingPage />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Bread' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(addShoppingItem).toHaveBeenCalled())
    expect(setItemCategory).not.toHaveBeenCalled()
  })

  it('creates a new category inline and assigns it on add', async () => {
    render(<ShoppingPage />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Kiwi' } })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: '__new__' } })
    fireEvent.change(screen.getByLabelText('New category name'), { target: { value: 'Fruit' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(createCategory).toHaveBeenCalledWith({ name: 'Fruit', ownerUserId: '' })
    )
    expect(setItemCategory).toHaveBeenCalledWith({
      name: 'Kiwi',
      categoryId: 'cat-new',
      ownerUserId: ''
    })
    expect(categoriesMutate).toHaveBeenCalled()
  })
})
