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
