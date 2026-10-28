import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

import { enqueueWrite } from '@/lib/offline/outbox'
import {
  createCategoryWrite,
  createShoppingItemWrite,
  deleteShoppingItemWrite,
  setItemCategoryWrite,
  updateShoppingItemWrite
} from '@/lib/shoppinglist/offlineWrites'

// Mutable meal-plan mock state (mock-prefixed for vi.mock); reset per test.
let mockMealExport: { data: { items: unknown[] }; isLoading: boolean } = {
  data: { items: [] },
  isLoading: false
}
let mockListLoading = false
let mockListError: Error | undefined
let mockPlanGroups: { data: { groups: unknown[] } } = { data: { groups: [] } }
let mockAliases: Record<string, string> = {}

vi.mock('@/hooks/useShoppingList', () => ({
  useCustomList: () => ({
    data: mockListError
      ? undefined
      : { items: [{ id: 'i1', name: 'Milk', amount: '1', unit: 'L' }] },
    error: mockListError,
    isLoading: mockListLoading
  }),
  useAllMealPlanExportItems: () => mockMealExport,
  useAllPlanIngredientGroups: () => mockPlanGroups,
  useStores: () => ({ data: { stores: [] }, isLoading: false }),
  useStoreCategories: () => ({ data: undefined, isLoading: false })
}))
vi.mock('@/hooks/useIngredientCatalog', () => ({
  useCategories: () => ({
    data: { categories: [{ id: 'cat-produce', name: 'Produce' }] }
  }),
  useItemCategories: () => ({ data: { items: [] }, isLoading: false }),
  useItemAliases: () => mockAliases
}))

vi.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: vi.fn().mockResolvedValue(undefined)
}))

const enqueue = vi.mocked(enqueueWrite)
const uuid = expect.stringMatching(/^[0-9a-f-]{36}$/)

import ShoppingListPageClient from '@/components/shoppinglist/ShoppingListPageClient'

beforeEach(() => {
  vi.clearAllMocks()
  mockMealExport = { data: { items: [] }, isLoading: false }
  mockPlanGroups = { data: { groups: [] } }
  mockListLoading = false
  mockListError = undefined
  mockAliases = {}
})

describe('ShoppingListPageClient load states', () => {
  it('shows an error, not an empty list, when the list fails to load', () => {
    mockListError = new Error('nope')
    render(<ShoppingListPageClient />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load shopping list.')
    expect(screen.queryByText(/No items yet/)).not.toBeInTheDocument()
  })
})

describe('ShoppingPage add form', () => {
  it('assigns the chosen category to the catalog on add', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.change(screen.getByRole('textbox', { name: 'New item name' }), {
      target: { value: 'Apples' }
    })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: 'cat-produce' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemCategoryWrite, {
        name: 'Apples',
        categoryId: 'cat-produce'
      })
    )
    expect(enqueue).toHaveBeenCalledWith(createShoppingItemWrite, {
      id: uuid,
      name: 'Apples',
      amount: '0',
      unit: ''
    })
  })

  it('skips the catalog write when no category is chosen', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Bread' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() => expect(screen.getByPlaceholderText('Item name')).toHaveValue(''))
    expect(enqueue).toHaveBeenCalledTimes(1)
    expect(enqueue).toHaveBeenCalledWith(createShoppingItemWrite, expect.anything())
  })

  it('creates a new category inline and assigns it on add', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Kiwi' } })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: '__new__' } })
    fireEvent.change(screen.getByLabelText('New category name'), { target: { value: 'Fruit' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemCategoryWrite, { name: 'Kiwi', categoryId: uuid })
    )
    const created: unknown = enqueue.mock.calls.find(([w]) => w === createCategoryWrite)?.[1]
    const assigned: unknown = enqueue.mock.calls.find(([w]) => w === setItemCategoryWrite)?.[1]
    expect(created).toEqual({ id: uuid, name: 'Fruit' })
    // The item's category is the client id just queued.
    expect(created).toHaveProperty('id', Reflect.get(Object(assigned), 'categoryId'))
  })

  it('reuses an existing category typed as new', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.change(screen.getByPlaceholderText('Item name'), { target: { value: 'Pear' } })
    fireEvent.change(screen.getByLabelText('Category'), { target: { value: '__new__' } })
    fireEvent.change(screen.getByLabelText('New category name'), { target: { value: 'produce' } })
    fireEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemCategoryWrite, {
        name: 'Pear',
        categoryId: 'cat-produce'
      })
    )
    expect(enqueue).not.toHaveBeenCalledWith(createCategoryWrite, expect.anything())
  })
})

describe('ShoppingPage edit', () => {
  it('queues an update of a custom item', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.click(screen.getByRole('button', { name: /Edit Milk/ }))
    fireEvent.change(screen.getByLabelText('Item name'), { target: { value: 'Oat Milk' } })
    fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('Unit'), { target: { value: 'cartons' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(updateShoppingItemWrite, {
        itemId: 'i1',
        name: 'Oat Milk',
        amount: '2',
        unit: 'cartons'
      })
    )
  })

  it('queues a delete of a custom item', async () => {
    render(<ShoppingListPageClient />)

    fireEvent.click(screen.getByRole('button', { name: /Remove Milk/ }))

    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(deleteShoppingItemWrite, { itemId: 'i1' })
    )
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
    // The group filter's checkbox label carries the group + recipe name.
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
    expect(screen.getByText('Order by store (optional)')).toBeInTheDocument()
    expect(screen.queryByText('Exclude ingredient groups')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(screen.queryByText('Export Shopping List')).not.toBeInTheDocument()
  })

  it('exports a merged custom item under its canonical name, keeping the typed name in the list', () => {
    mockAliases = { milk: 'melk' }
    mockMealExport = {
      data: { items: [{ name: 'melk', amount: '2', unit: 'L', recipeName: 'Pap' }] },
      isLoading: false
    }
    render(<ShoppingListPageClient />)

    expect(screen.getByText(/1 L - Milk/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    expect(screen.getByText(/3 L — melk/)).toBeInTheDocument()
  })
})

describe('ShoppingPage header and loading', () => {
  it('links to settings from the page header', () => {
    render(<ShoppingListPageClient />)
    expect(screen.getByRole('heading', { level: 1, name: 'Shopping List' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute(
      'href',
      '/shoppinglist/settings'
    )
  })

  it('shows a loading state while the list loads', () => {
    mockListLoading = true
    render(<ShoppingListPageClient />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading…')
  })
})
