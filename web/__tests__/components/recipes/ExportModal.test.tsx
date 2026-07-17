import React from 'react'
import { render, screen, fireEvent, act } from '@testing-library/react'
import ExportModal from '@/components/recipes/ExportModal'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

// The modal no longer fetches meal-plan items or ingredient groups — those are
// owned by the landing page and passed in via the mealItems prop.
jest.mock('@/hooks/useShoppingList', () => ({
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
const mealItems: ShoppingItem[] = [
  { name: 'garlic', amount: '2', unit: 'cloves', recipeName: 'Pasta', groupName: 'Sauce' }
]

const renderModal = (props: Partial<React.ComponentProps<typeof ExportModal>> = {}) =>
  render(
    <ExportModal customItems={customItems} mealItems={mealItems} onClose={jest.fn()} {...props} />
  )

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
    renderModal()
    expect(screen.getByRole('button', { name: /Copy to Clipboard/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Share to Apple Notes/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Download .txt/ })).toBeInTheDocument()
  })

  it('shows the passed-in meal plan items in the export preview', () => {
    renderModal()
    expect(screen.getByText(/2 cloves — garlic/)).toBeInTheDocument()
  })

  it('no longer renders the ingredient-group exclusion controls', () => {
    renderModal()
    expect(screen.queryByText('Exclude ingredient groups')).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  })

  it('shows the ingredient group name in the origin label in the preview', () => {
    renderModal({ customItems: [] })
    expect(screen.getByText(/Pasta \[Sauce\]/)).toBeInTheDocument()
  })

  it('renders store selector with options', () => {
    renderModal()
    expect(screen.getByRole('option', { name: 'Colruyt' })).toBeInTheDocument()
  })

  it('groups items by store aisle order when a store is selected', () => {
    renderModal()
    fireEvent.change(screen.getByLabelText('Order by store (optional)'), {
      target: { value: 'store-1' }
    })
    expect(screen.getByText('Grouped by store aisle')).toBeInTheDocument()
    expect(screen.getByText('Dairy')).toBeInTheDocument()
    expect(screen.getByText(/1 L — milk/)).toBeInTheDocument()
  })

  it('warns when items have no category assigned for the selected store', () => {
    renderModal()
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
    renderModal({ customItems: items })
    fireEvent.change(screen.getByLabelText('Order by store (optional)'), {
      target: { value: 'store-1' }
    })
    // icecream → cat-frozen, which store-1 does not order
    expect(screen.getByText(/this store doesn.t order/)).toBeInTheDocument()
  })

  it('does not show store-coverage warnings until a store is selected', () => {
    renderModal()
    expect(screen.queryByText(/no category assigned/)).not.toBeInTheDocument()
    expect(screen.queryByText(/this store doesn.t order/)).not.toBeInTheDocument()
  })

  it('copies grouped output to clipboard when a store is selected', async () => {
    renderModal()
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
    renderModal({ onClose })
    fireEvent.click(screen.getByRole('button', { name: /Close/ }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when Escape is pressed', () => {
    const onClose = jest.fn()
    renderModal({ onClose })
    fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('Copy to Clipboard calls navigator.clipboard.writeText', async () => {
    renderModal()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Copy to Clipboard/ }))
    })
    expect(navigator.clipboard.writeText).toHaveBeenCalled()
  })

  it('Share to Apple Notes falls back to clipboard when navigator.share is absent', async () => {
    renderModal()
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
    renderModal()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Share to Apple Notes/ }))
    })
    expect(mockShare).toHaveBeenCalled()
  })

  it('Share to Apple Notes does not throw when user cancels the share sheet', async () => {
    const abortError = new DOMException('Share cancelled', 'AbortError')
    const mockShare = jest.fn().mockRejectedValue(abortError)
    Object.defineProperty(navigator, 'share', {
      value: mockShare,
      writable: true,
      configurable: true
    })
    renderModal()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Share to Apple Notes/ }))
    })
    expect(mockShare).toHaveBeenCalled()
    // no error thrown
  })

  it('Download .txt triggers file download', () => {
    const mockClick = jest.fn()
    renderModal()
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
