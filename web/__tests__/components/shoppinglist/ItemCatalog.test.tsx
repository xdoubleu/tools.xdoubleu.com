import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ItemCatalog from '@/components/shoppinglist/ItemCatalog'
import { enqueueWrite } from '@/lib/offline/outbox'
import { setItemCategoryWrite, setItemExcludedWrite } from '@/lib/shoppinglist/offlineWrites'

type Name = { name: string; categoryId: string; excluded: boolean }
let namesData: { names: Name[] } | undefined

const defaultNames: Name[] = [
  { name: 'milk', categoryId: 'cat-dairy', excluded: false },
  { name: 'apple', categoryId: '', excluded: false },
  { name: 'cake', categoryId: '', excluded: true }
]

jest.mock('@/hooks/useShoppingList', () => ({
  useItemNames: () => ({ data: namesData, isLoading: false }),
  useCategories: () => ({
    data: {
      categories: [
        { id: 'cat-dairy', name: 'Dairy' },
        { id: 'cat-produce', name: 'Produce' }
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
  namesData = { names: defaultNames.map((n) => ({ ...n })) }
})

describe('ItemCatalog', () => {
  it('groups items by category with an Unassigned group', () => {
    render(<ItemCatalog />)
    expect(screen.getByRole('button', { name: /Unassigned/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Dairy/ })).toBeInTheDocument()
    expect(screen.getByText('milk')).toBeInTheDocument()
    expect(screen.getByText('apple')).toBeInTheDocument()
  })

  it('assigns a category to an item name', async () => {
    render(<ItemCatalog />)
    fireEvent.change(screen.getByLabelText('Category for apple'), {
      target: { value: 'cat-produce' }
    })
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemCategoryWrite, {
        name: 'apple',
        categoryId: 'cat-produce'
      })
    )
  })

  it('removes an item from the export when its toggle is unchecked', async () => {
    render(<ItemCatalog />)
    fireEvent.click(screen.getByLabelText('Export milk to list'))
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemExcludedWrite, { name: 'milk', excluded: true })
    )
  })

  it('shows excluded items under a collapsed "Not exported" group and restores them', async () => {
    render(<ItemCatalog />)
    // Collapsed by default: the excluded item is not visible yet.
    expect(screen.queryByText('cake')).not.toBeInTheDocument()

    fireEvent.click(screen.getByText('Not exported'))
    expect(screen.getByText('cake')).toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('Export cake to list'))
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemExcludedWrite, { name: 'cake', excluded: false })
    )
  })

  it('renders the empty state when there are no item names', () => {
    namesData = { names: [] }
    render(<ItemCatalog />)
    expect(screen.getByText(/No items yet/)).toBeInTheDocument()
  })
})
