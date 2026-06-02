import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ItemCatalog from '@/components/recipes/ItemCatalog'

const mutate = jest.fn().mockResolvedValue(undefined)
const setItemCategory = jest.fn().mockResolvedValue({})

let namesData: { names: { name: string; categoryId: string }[] } | undefined = {
  names: [
    { name: 'milk', categoryId: 'cat-dairy' },
    { name: 'apple', categoryId: '' }
  ]
}

jest.mock('@/hooks/useShoppingList', () => ({
  useItemNames: () => ({ data: namesData, isLoading: false, mutate }),
  useCategories: () => ({
    data: {
      categories: [
        { id: 'cat-dairy', name: 'Dairy' },
        { id: 'cat-produce', name: 'Produce' }
      ]
    }
  })
}))

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({ setItemCategory })
}))

beforeEach(() => {
  jest.clearAllMocks()
  namesData = {
    names: [
      { name: 'milk', categoryId: 'cat-dairy' },
      { name: 'apple', categoryId: '' }
    ]
  }
})

describe('ItemCatalog', () => {
  it('lists item names and flags unassigned ones', () => {
    render(<ItemCatalog />)
    expect(screen.getByText('milk')).toBeInTheDocument()
    expect(screen.getByText('apple')).toBeInTheDocument()
    expect(screen.getByText('unassigned')).toBeInTheDocument()
  })

  it('assigns a category to an item name', async () => {
    render(<ItemCatalog />)
    fireEvent.change(screen.getByLabelText('Category for apple'), {
      target: { value: 'cat-produce' }
    })
    await waitFor(() =>
      expect(setItemCategory).toHaveBeenCalledWith({ name: 'apple', categoryId: 'cat-produce' })
    )
    expect(mutate).toHaveBeenCalled()
  })

  it('renders the empty state when there are no item names', () => {
    namesData = { names: [] }
    render(<ItemCatalog />)
    expect(screen.getByText(/No items yet/)).toBeInTheDocument()
  })
})
