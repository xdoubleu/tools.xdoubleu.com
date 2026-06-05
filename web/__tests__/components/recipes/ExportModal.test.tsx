import React from 'react'
import { render, screen, fireEvent, act } from '@testing-library/react'
import ExportModal from '@/components/recipes/ExportModal'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

jest.mock('@/hooks/useShoppingList', () => ({
  useAllMealPlanExportItems: (excludedGroups: string[]) => ({
    data: {
      items: excludedGroups.includes('Sauce')
        ? []
        : [{ name: 'garlic', amount: '2', unit: 'cloves', recipeName: 'Pasta', groupName: 'Sauce' }]
    },
    isLoading: false
  }),
  useAllPlanIngredientGroups: () => ({
    data: { groups: [{ recipeName: 'Pasta', groupName: 'Sauce' }] },
    isLoading: false
  }),
  useStores: () => ({
    data: { stores: [{ id: 'store-1', name: 'Colruyt' }] },
    isLoading: false
  }),
  useStoreCategories: (storeId: string) => ({
    data: storeId
      ? {
          categories: [
            { id: 'cat-veg', name: 'Vegetables' },
            { id: 'cat-dairy', name: 'Dairy' }
          ]
        }
      : undefined,
    isLoading: false
  }),
  useItemCategories: () => ({
    data: {
      items: [
        { name: 'milk', categoryId: 'cat-dairy' },
        // cat-frozen is a real category, but store-1 does not order it
        { name: 'icecream', categoryId: 'cat-frozen' }
      ]
    },
    isLoading: false
  })
}))

const customItems: ShoppingItem[] = [{ id: 'c1', amount: '1', unit: 'L', name: 'milk' }]

beforeEach(() => {
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: jest.fn().mockResolvedValue(undefined) },
    writable: true,
    configurable: true
  })
  Object.defineProperty(navigator, 'share', {
    value: undefined,
    writable: true,
    configurable: true
  })
})

describe('ExportModal', () => {
  it('renders export buttons', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.getByRole('button', { name: /Copy to Clipboard/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Share to Apple Notes/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Download .txt/ })).toBeInTheDocument()
  })

  it('does not render a plan selector dropdown', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.queryByLabelText(/Add meal plan ingredients/)).not.toBeInTheDocument()
  })

  it('always shows meal plan items in the export preview', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.getByText(/2 cloves — garlic/)).toBeInTheDocument()
  })

  it('shows ingredient groups for exclusion', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.getByText('Sauce')).toBeInTheDocument()
    expect(screen.getByText('(Pasta)')).toBeInTheDocument()
  })

  it('excludes ingredient group items when group is unchecked', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    const checkbox = screen.getByRole('checkbox')
    expect(checkbox).toBeChecked()
    fireEvent.click(checkbox)
    // After excluding 'Sauce', garlic (from Sauce group) should be gone
    expect(screen.queryByText(/garlic/)).not.toBeInTheDocument()
  })

  it('shows ingredient group name in origin label in the preview', () => {
    render(<ExportModal customItems={[]} onClose={jest.fn()} />)
    expect(screen.getByText(/Pasta \[Sauce\]/)).toBeInTheDocument()
  })

  it('renders store selector with options', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.getByRole('option', { name: 'Colruyt' })).toBeInTheDocument()
  })

  it('groups items by store aisle order when a store is selected', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Order by store (optional)'), {
      target: { value: 'store-1' }
    })
    expect(screen.getByText('Grouped by store aisle')).toBeInTheDocument()
    expect(screen.getByText('Dairy')).toBeInTheDocument()
    expect(screen.getByText(/1 L — milk/)).toBeInTheDocument()
  })

  it('warns when items have no category assigned for the selected store', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Order by store (optional)'), {
      target: { value: 'store-1' }
    })
    // garlic (from the meal plan) maps to no category at all — exactly one item,
    // so the message must read "1 item has" with the space intact (not "1 itemhas")
    expect(
      screen.getByText('1 item has no category assigned and will appear under "Other".')
    ).toBeInTheDocument()
  })

  it('warns when an item has a category the selected store does not order', () => {
    const items: ShoppingItem[] = [
      ...customItems,
      { id: 'c2', amount: '1', unit: 'tub', name: 'icecream' }
    ]
    render(<ExportModal customItems={items} onClose={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Order by store (optional)'), {
      target: { value: 'store-1' }
    })
    // icecream → cat-frozen, which store-1 does not order
    expect(screen.getByText(/this store doesn.t order/)).toBeInTheDocument()
  })

  it('does not show store-coverage warnings until a store is selected', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.queryByText(/no category assigned/)).not.toBeInTheDocument()
    expect(screen.queryByText(/this store doesn.t order/)).not.toBeInTheDocument()
  })

  it('copies grouped output to clipboard when a store is selected', async () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    fireEvent.change(screen.getByLabelText('Order by store (optional)'), {
      target: { value: 'store-1' }
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Copy to Clipboard/ }))
    })
    // milk → Dairy; garlic → Other (not in nameToCategoryId)
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      expect.stringContaining('Dairy:\n1 L - milk')
    )
  })

  it('calls onClose when close button is clicked', () => {
    const onClose = jest.fn()
    render(<ExportModal customItems={customItems} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: /Close/ }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when Escape is pressed', () => {
    const onClose = jest.fn()
    render(<ExportModal customItems={customItems} onClose={onClose} />)
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('Copy to Clipboard calls navigator.clipboard.writeText', async () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Copy to Clipboard/ }))
    })
    expect(navigator.clipboard.writeText).toHaveBeenCalled()
  })

  it('Share to Apple Notes falls back to clipboard when navigator.share is absent', async () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Share to Apple Notes/ }))
    })
    expect(navigator.clipboard.writeText).toHaveBeenCalled()
  })

  it('Share to Apple Notes calls navigator.share when available', async () => {
    const mockShare = jest.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'share', {
      value: mockShare,
      writable: true,
      configurable: true
    })
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Share to Apple Notes/ }))
    })
    expect(mockShare).toHaveBeenCalled()
  })

  it('Download .txt triggers file download', () => {
    const mockClick = jest.fn()
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    const mockAppendChild = jest.spyOn(document.body, 'appendChild').mockImplementation(jest.fn())
    const mockRemoveChild = jest.spyOn(document.body, 'removeChild').mockImplementation(jest.fn())
    const realCreate = document.createElement.bind(document)
    jest.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      if (tag === 'a') {
        const el = realCreate('a')
        el.click = mockClick
        return el
      }
      return realCreate(tag)
    })
    fireEvent.click(screen.getByRole('button', { name: /Download .txt/ }))
    expect(mockClick).toHaveBeenCalled()
    mockAppendChild.mockRestore()
    mockRemoveChild.mockRestore()
    jest.restoreAllMocks()
  })
})
