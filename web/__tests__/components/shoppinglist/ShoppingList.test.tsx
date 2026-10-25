import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ShoppingList from '@/components/shoppinglist/ShoppingList'
import type { ShoppingItem } from '@/lib/shoppinglist/shoppingExport'

describe('ShoppingList', () => {
  const items: ShoppingItem[] = [
    { amount: '2', unit: 'cups', name: 'flour' },
    { id: 'custom-1', amount: '1', unit: 'L', name: 'milk' }
  ]

  beforeEach(() => localStorage.clear())

  it('renders empty state when items list is empty', () => {
    render(<ShoppingList items={[]} />)
    expect(screen.getByText('No items yet. Add something above.')).toBeInTheDocument()
  })

  it('renders items', () => {
    render(<ShoppingList items={items} />)
    expect(screen.getByText(/2 cups - flour/)).toBeInTheDocument()
    expect(screen.getByText(/1 L - milk/)).toBeInTheDocument()
  })

  it('toggles item checked state', () => {
    render(<ShoppingList items={items} />)
    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[0])
    expect(checkboxes[0]).toBeChecked()
  })

  describe('ticks', () => {
    const TICKS_KEY = 'shoppinglist:ticks'
    const eggs: ShoppingItem = { id: 'custom-2', amount: '6', unit: '', name: 'eggs' }
    const milkBox = () => screen.getByRole('checkbox', { name: /milk/ })

    afterEach(() => vi.restoreAllMocks())

    it('stay on the same item when items are added above it', () => {
      const { rerender } = render(<ShoppingList items={items} />)
      fireEvent.click(milkBox())

      rerender(<ShoppingList items={[eggs, ...items]} />)

      expect(milkBox()).toBeChecked()
      expect(screen.getByRole('checkbox', { name: /eggs/ })).not.toBeChecked()
    })

    it('survive a reload', () => {
      const { unmount } = render(<ShoppingList items={items} />)
      fireEvent.click(milkBox())
      unmount()

      render(<ShoppingList items={items} />)
      expect(milkBox()).toBeChecked()
    })

    it('drop ids of items that no longer exist', () => {
      localStorage.setItem(TICKS_KEY, JSON.stringify(['gone', 'custom-1']))
      render(<ShoppingList items={[items[1], eggs]} />)
      expect(milkBox()).toBeChecked()

      fireEvent.click(screen.getByRole('checkbox', { name: /eggs/ }))

      expect(JSON.parse(localStorage.getItem(TICKS_KEY)!)).toEqual(['custom-1', 'custom-2'])
    })

    it('ignore a stored value that is not a list', () => {
      localStorage.setItem(TICKS_KEY, JSON.stringify({ 'custom-1': true }))
      render(<ShoppingList items={items} />)
      expect(milkBox()).not.toBeChecked()
      fireEvent.click(milkBox())
      expect(milkBox()).toBeChecked()
    })

    it('still toggle when storage is unavailable', () => {
      vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
        throw new Error('denied')
      })
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('denied')
      })
      render(<ShoppingList items={items} />)
      fireEvent.click(milkBox())
      expect(milkBox()).toBeChecked()
    })
  })

  describe('delete button', () => {
    it('does not render delete buttons when onDelete is not provided', () => {
      render(<ShoppingList items={items} />)
      expect(screen.queryByRole('button', { name: /Remove/ })).not.toBeInTheDocument()
    })

    it('renders delete button only for items that have an id', () => {
      const onDelete = vi.fn()
      render(<ShoppingList items={items} onDelete={onDelete} />)
      expect(screen.queryByRole('button', { name: /Remove flour/ })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Remove milk/ })).toBeInTheDocument()
    })

    it('calls onDelete with the item id when delete button is clicked', async () => {
      const onDelete = vi.fn().mockResolvedValue(undefined)
      render(<ShoppingList items={items} onDelete={onDelete} />)
      fireEvent.click(screen.getByRole('button', { name: /Remove milk/ }))
      await waitFor(() => expect(onDelete).toHaveBeenCalledWith('custom-1'))
    })
  })

  describe('edit button', () => {
    it('does not render edit buttons when onEdit is not provided', () => {
      render(<ShoppingList items={items} />)
      expect(screen.queryByRole('button', { name: /Edit/ })).not.toBeInTheDocument()
    })

    it('renders edit button only for items that have an id', () => {
      render(<ShoppingList items={items} onEdit={vi.fn()} />)
      expect(screen.queryByRole('button', { name: /Edit flour/ })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Edit milk/ })).toBeInTheDocument()
    })

    it('shows an inline edit form prefilled with the item values', () => {
      render(<ShoppingList items={items} onEdit={vi.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: /Edit milk/ }))
      expect(screen.getByLabelText('Item name')).toHaveValue('milk')
      expect(screen.getByLabelText('Amount')).toHaveValue(1)
      expect(screen.getByLabelText('Unit')).toHaveValue('L')
    })

    it('calls onEdit with the item id and edited values when saved', async () => {
      const onEdit = vi.fn().mockResolvedValue(undefined)
      render(<ShoppingList items={items} onEdit={onEdit} />)
      fireEvent.click(screen.getByRole('button', { name: /Edit milk/ }))
      fireEvent.change(screen.getByLabelText('Item name'), { target: { value: 'oat milk' } })
      fireEvent.change(screen.getByLabelText('Amount'), { target: { value: '2' } })
      fireEvent.change(screen.getByLabelText('Unit'), { target: { value: 'cartons' } })
      fireEvent.click(screen.getByRole('button', { name: 'Save' }))
      await waitFor(() =>
        expect(onEdit).toHaveBeenCalledWith('custom-1', {
          name: 'oat milk',
          amount: '2',
          unit: 'cartons'
        })
      )
    })

    it('does not call onEdit when the name is blank', () => {
      const onEdit = vi.fn()
      render(<ShoppingList items={items} onEdit={onEdit} />)
      fireEvent.click(screen.getByRole('button', { name: /Edit milk/ }))
      fireEvent.change(screen.getByLabelText('Item name'), { target: { value: '   ' } })
      fireEvent.click(screen.getByRole('button', { name: 'Save' }))
      expect(onEdit).not.toHaveBeenCalled()
    })

    it('cancels editing without calling onEdit', () => {
      const onEdit = vi.fn()
      render(<ShoppingList items={items} onEdit={onEdit} />)
      fireEvent.click(screen.getByRole('button', { name: /Edit milk/ }))
      fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      expect(onEdit).not.toHaveBeenCalled()
      expect(screen.getByText(/1 L - milk/)).toBeInTheDocument()
    })
  })

  describe('export button', () => {
    it('does not render export button when onExport is not provided', () => {
      render(<ShoppingList items={items} />)
      expect(screen.queryByRole('button', { name: /Export/ })).not.toBeInTheDocument()
    })

    it('renders export button when onExport is provided', () => {
      render(<ShoppingList items={items} onExport={vi.fn()} />)
      expect(screen.getByRole('button', { name: /Export/ })).toBeInTheDocument()
    })

    it('calls onExport when export button is clicked', () => {
      const onExport = vi.fn()
      render(<ShoppingList items={items} onExport={onExport} />)
      fireEvent.click(screen.getByRole('button', { name: /Export/ }))
      expect(onExport).toHaveBeenCalledTimes(1)
    })

    it('renders export button in empty state when onExport is provided', () => {
      render(<ShoppingList items={[]} onExport={vi.fn()} />)
      expect(screen.getByRole('button', { name: /Export/ })).toBeInTheDocument()
    })
  })
})
