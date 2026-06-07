import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const addShoppingItem = jest.fn().mockResolvedValue({ item: { id: 'i1' } })
const setItemCategory = jest.fn().mockResolvedValue({})
const deleteShoppingItem = jest.fn().mockResolvedValue({})
const listMutate = jest.fn().mockResolvedValue(undefined)
const globalMutate = jest.fn().mockResolvedValue(undefined)

jest.mock('@/hooks/useShoppingList', () => ({
  useCustomList: () => ({ data: { items: [] }, isLoading: false, mutate: listMutate }),
  useCategories: () => ({ data: { categories: [{ id: 'cat-produce', name: 'Produce' }] } })
}))

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ addShoppingItem, setItemCategory, deleteShoppingItem })
}))

jest.mock('swr', () => ({
  __esModule: true,
  useSWRConfig: () => ({ mutate: globalMutate })
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
      expect(addShoppingItem).toHaveBeenCalledWith({ name: 'Apples', amount: '0', unit: '' })
    )
    expect(setItemCategory).toHaveBeenCalledWith({ name: 'Apples', categoryId: 'cat-produce' })
    expect(globalMutate).toHaveBeenCalledWith('/shoppinglist/item-categories')
  })

  it('skips the catalog write when no category is chosen', async () => {
    render(<ShoppingPage />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Bread' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(addShoppingItem).toHaveBeenCalled())
    expect(setItemCategory).not.toHaveBeenCalled()
  })
})
