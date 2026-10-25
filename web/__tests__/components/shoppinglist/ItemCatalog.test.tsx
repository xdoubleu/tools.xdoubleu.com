import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ItemCatalog from '@/components/shoppinglist/ItemCatalog'
import { enqueueWrite } from '@/lib/offline/outbox'
import {
  setItemAliasWrite,
  setItemCategoryWrite,
  setItemExcludedWrite
} from '@/lib/shoppinglist/offlineWrites'

type Name = { name: string; categoryId: string; excluded: boolean; mergedInto: string }
let namesData: { names: Name[] } | undefined
let namesError: Error | undefined

const defaultNames: Name[] = [
  { name: 'milk', categoryId: 'cat-dairy', excluded: false, mergedInto: '' },
  { name: 'apple', categoryId: '', excluded: false, mergedInto: '' },
  { name: 'cake', categoryId: '', excluded: true, mergedInto: '' },
  { name: 'rode ui', categoryId: '', excluded: false, mergedInto: 'apple' }
]

vi.mock('@/hooks/useShoppingList', () => ({
  useItemNames: () => ({ data: namesData, error: namesError, isLoading: false }),
  useCategories: () => ({
    data: {
      categories: [
        { id: 'cat-dairy', name: 'Dairy' },
        { id: 'cat-produce', name: 'Produce' }
      ]
    }
  })
}))

vi.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: vi.fn().mockResolvedValue(undefined)
}))

const enqueue = vi.mocked(enqueueWrite)

beforeEach(() => {
  vi.clearAllMocks()
  namesData = { names: defaultNames.map((n) => ({ ...n })) }
  namesError = undefined
})

describe('ItemCatalog', () => {
  it('shows an error, not the empty state, when the items fail to load', () => {
    namesData = undefined
    namesError = new Error('nope')
    render(<ItemCatalog />)
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load items.')
    expect(screen.queryByText(/No items yet/)).not.toBeInTheDocument()
  })

  it('groups items by category with an Unassigned group', () => {
    render(<ItemCatalog />)
    expect(screen.getByRole('button', { name: /Unassigned/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Dairy/ })).toBeInTheDocument()
    expect(screen.getByText('milk', { selector: 'span' })).toBeInTheDocument()
    expect(screen.getByText('apple', { selector: 'span' })).toBeInTheDocument()
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

  it('offers only exported, unmerged names as merge targets', () => {
    render(<ItemCatalog />)
    const options = Array.from(
      (screen.getByLabelText('Merge apple into') as HTMLSelectElement).options
    ).map((o) => o.value)
    expect(options).toEqual(['', 'milk'])
  })

  it('merges an item into another', async () => {
    render(<ItemCatalog />)
    fireEvent.change(screen.getByLabelText('Merge milk into'), { target: { value: 'apple' } })
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemAliasWrite, { name: 'milk', canonical: 'apple' })
    )
  })

  it('shows merged items under a collapsed "Merged" group and unmerges them', async () => {
    render(<ItemCatalog />)
    expect(screen.queryByText('rode ui')).not.toBeInTheDocument()

    fireEvent.click(screen.getByText('Merged'))
    expect(screen.getByText('rode ui')).toBeInTheDocument()
    expect(screen.getByText('→ apple')).toBeInTheDocument()
    expect(screen.queryByLabelText('Category for rode ui')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Unmerge rode ui' }))
    await waitFor(() =>
      expect(enqueue).toHaveBeenCalledWith(setItemAliasWrite, { name: 'rode ui', canonical: '' })
    )
  })

  it('renders the empty state when there are no item names', () => {
    namesData = { names: [] }
    render(<ItemCatalog />)
    expect(screen.getByText(/No items yet/)).toBeInTheDocument()
  })
})
