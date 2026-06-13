import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ShoppingList from '@/components/recipes/ShoppingList'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

describe('ShoppingList', () => {
  const items: ShoppingItem[] = [
    { amount: '2', unit: 'cups', name: 'flour' },
    { id: 'custom-1', amount: '1', unit: 'L', name: 'milk' }
  ]

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

  describe('delete button', () => {
    it('does not render delete buttons when onDelete is not provided', () => {
      render(<ShoppingList items={items} />)
      expect(screen.queryByRole('button', { name: /Remove/ })).not.toBeInTheDocument()
    })

    it('renders delete button only for items that have an id', () => {
      const onDelete = jest.fn()
      render(<ShoppingList items={items} onDelete={onDelete} />)
      expect(screen.queryByRole('button', { name: /Remove flour/ })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Remove milk/ })).toBeInTheDocument()
    })

    it('calls onDelete with the item id when delete button is clicked', async () => {
      const onDelete = jest.fn().mockResolvedValue(undefined)
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
      render(<ShoppingList items={items} onEdit={jest.fn()} />)
      expect(screen.queryByRole('button', { name: /Edit flour/ })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Edit milk/ })).toBeInTheDocument()
    })

    it('shows an inline edit form prefilled with the item values', () => {
      render(<ShoppingList items={items} onEdit={jest.fn()} />)
      fireEvent.click(screen.getByRole('button', { name: /Edit milk/ }))
      expect(screen.getByLabelText('Item name')).toHaveValue('milk')
      expect(screen.getByLabelText('Amount')).toHaveValue(1)
      expect(screen.getByLabelText('Unit')).toHaveValue('L')
    })

    it('calls onEdit with the item id and edited values when saved', async () => {
      const onEdit = jest.fn().mockResolvedValue(undefined)
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
      const onEdit = jest.fn()
      render(<ShoppingList items={items} onEdit={onEdit} />)
      fireEvent.click(screen.getByRole('button', { name: /Edit milk/ }))
      fireEvent.change(screen.getByLabelText('Item name'), { target: { value: '   ' } })
      fireEvent.click(screen.getByRole('button', { name: 'Save' }))
      expect(onEdit).not.toHaveBeenCalled()
    })

    it('cancels editing without calling onEdit', () => {
      const onEdit = jest.fn()
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
      render(<ShoppingList items={items} onExport={jest.fn()} />)
      expect(screen.getByRole('button', { name: /Export/ })).toBeInTheDocument()
    })

    it('calls onExport when export button is clicked', () => {
      const onExport = jest.fn()
      render(<ShoppingList items={items} onExport={onExport} />)
      fireEvent.click(screen.getByRole('button', { name: /Export/ }))
      expect(onExport).toHaveBeenCalledTimes(1)
    })

    it('renders export button in empty state when onExport is provided', () => {
      render(<ShoppingList items={[]} onExport={jest.fn()} />)
      expect(screen.getByRole('button', { name: /Export/ })).toBeInTheDocument()
    })
  })
})
