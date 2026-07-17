import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const createShoppingItem = jest.fn().mockResolvedValue({ item: { id: 'i1' } })
const setItemCategory = jest.fn().mockResolvedValue({})
const createCategory = jest.fn().mockResolvedValue({ category: { id: 'cat-new' } })
const deleteShoppingItem = jest.fn().mockResolvedValue({})
const updateShoppingItem = jest.fn().mockResolvedValue({ item: { id: 'i1' } })
const listMutate = jest.fn().mockResolvedValue(undefined)
const categoriesMutate = jest.fn().mockResolvedValue(undefined)

// Mutable meal-plan mock state (mock-prefixed so jest.mock's factory may close
// over it); reset in beforeEach and overridden per-test.
let mockMealExport: { data: { items: unknown[] }; isLoading: boolean } = {
  data: { items: [] },
  isLoading: false
}
let mockPlanGroups: { data: { groups: unknown[] } } = { data: { groups: [] } }

jest.mock('@/hooks/useShoppingList', () => ({
  useCustomList: () => ({
    data: { items: [{ id: 'i1', name: 'Milk', amount: '1', unit: 'L' }] },
    isLoading: false,
    mutate: listMutate
  }),
  useCategories: () => ({
    data: { categories: [{ id: 'cat-produce', name: 'Produce' }] },
    mutate: categoriesMutate
  }),
  useAccessibleLists: () => ({ data: { owners: [] } }),
  useShoppingListShares: () => ({ data: { shares: [] }, mutate: jest.fn() }),
  useShareShoppingList: () => jest.fn().mockResolvedValue(undefined),
  useUnshareShoppingList: () => jest.fn().mockResolvedValue(undefined),
  useAllMealPlanExportItems: () => mockMealExport,
  useAllPlanIngredientGroups: () => mockPlanGroups,
  // Consumed by ExportModal once the export dialog is opened.
  useStores: () => ({ data: { stores: [] }, isLoading: false }),
  useStoreCategories: () => ({ data: undefined, isLoading: false }),
  useItemCategories: () => ({ data: { items: [] }, isLoading: false })
}))

jest.mock('@/lib/client', () => ({
  createServiceClient: () => ({
    createShoppingItem,
    setItemCategory,
    createCategory,
    deleteShoppingItem,
    updateShoppingItem
  })
}))

import ShoppingListPageClient from '@/components/shoppinglist/ShoppingListPageClient'

beforeEach(() => {
  jest.clearAllMocks()
  mockMealExport = { data: { items: [] }, isLoading: false }
  mockPlanGroups = { data: { groups: [] } }
})

describe('ShoppingPage add form', () => {
  it('assigns the chosen category to the catalog on add', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Apples' } })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'cat-produce' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(createShoppingItem).toHaveBeenCalledWith({
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
    render(<ShoppingListPageClient />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Bread' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(createShoppingItem).toHaveBeenCalled())
    expect(setItemCategory).not.toHaveBeenCalled()
  })

  it('creates a new category inline and assigns it on add', async () => {
    render(<ShoppingListPageClient />)

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

describe('ShoppingPage edit', () => {
  it('updates a custom item and refreshes the list', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.click(screen.getByRole('button', { name: /Edit Milk/ }))
    fireEvent.change(screen.getByLabelText('Item name'), { target: { value: 'Oat Milk' } })
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('Unit'), { target: { value: 'cartons' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateShoppingItem).toHaveBeenCalledWith({
        itemId: 'i1',
        name: 'Oat Milk',
        amount: '2',
        unit: 'cartons',
        ownerUserId: ''
      })
    )
    expect(listMutate).toHaveBeenCalled()
  })
})

describe('ShoppingPage meal-plan section', () => {
  it('shows meal-plan items read-only (no edit/delete controls)', () => {
    mockMealExport = {
      data: {
        items: [
          { name: 'garlic', amount: '2', unit: 'cloves', recipeName: 'Pasta', groupName: 'Sauce' }
        ]
      },
      isLoading: false
    }
    render(<ShoppingListPageClient />)

    expect(screen.getByText('From meal plans')).toBeInTheDocument()
    expect(screen.getByText(/2 cloves — garlic/)).toBeInTheDocument()
    // The custom item still has an Edit button; the meal-plan item never does.
    expect(screen.queryByRole('button', { name: /Edit garlic/ })).not.toBeInTheDocument()
  })

  it('hides the meal-plan section when there are no meal-plan items', () => {
    render(<ShoppingListPageClient />)
    expect(screen.queryByText('From meal plans')).not.toBeInTheDocument()
  })

  it('renders the ingredient-group filter and toggles a group off', () => {
    mockPlanGroups = { data: { groups: [{ recipeName: 'Pasta', groupName: 'Sauce' }] } }
    render(<ShoppingListPageClient />)

    expect(screen.getByText('Exclude ingredient groups')).toBeInTheDocument()
    // The editable list also renders per-item checkboxes; the group filter's is
    // the one whose <label> carries the group + recipe name.
    const checkbox = screen.getByRole('checkbox', { name: /Sauce/ })
    expect(checkbox).toBeChecked()
    fireEvent.click(checkbox)
    expect(checkbox).not.toBeChecked()
  })

  it('hides the group filter when no ingredient groups exist', () => {
    render(<ShoppingListPageClient />)
    expect(screen.queryByText('Exclude ingredient groups')).not.toBeInTheDocument()
  })

  it('opens the export dialog (store-only) with the meal items passed in', () => {
    mockMealExport = {
      data: {
        items: [
          { name: 'garlic', amount: '2', unit: 'cloves', recipeName: 'Pasta', groupName: 'Sauce' }
        ]
      },
      isLoading: false
    }
    render(<ShoppingListPageClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    expect(screen.getByText('Export Shopping List')).toBeInTheDocument()
    // The meal item flows into the modal's preview, and the modal no longer owns
    // the ingredient-group controls.
    expect(screen.getByText('Order by store (optional)')).toBeInTheDocument()
    expect(screen.queryByText('Exclude ingredient groups')).not.toBeInTheDocument()
  })
})
