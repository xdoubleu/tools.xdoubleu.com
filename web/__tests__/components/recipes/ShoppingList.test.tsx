import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import ShoppingList from '@/components/recipes/ShoppingList'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

describe('ShoppingList', () => {
  const mockItems: ShoppingItem[] = [
    { amount: '2', unit: 'cups', name: 'flour' },
    { amount: '1', unit: 'tbsp', name: 'sugar' }
  ]

  it('renders empty state when no items', () => {
    render(<ShoppingList items={[]} />)
    expect(screen.getByText('No shopping items.')).toBeInTheDocument()
  })

  it('renders all items', () => {
    render(<ShoppingList items={mockItems} />)
    expect(screen.getByText(/2 cups - flour/)).toBeInTheDocument()
    expect(screen.getByText(/1 tbsp - sugar/)).toBeInTheDocument()
  })

  it('toggles item checked state', () => {
    render(<ShoppingList items={mockItems} />)
    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[0])
    expect(checkboxes[0]).toBeChecked()
  })

  it('renders export buttons', () => {
    render(<ShoppingList items={mockItems} />)
    expect(screen.getByRole('button', { name: /Copy to Clipboard/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Copy.*Apple Notes/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Download .txt/ })).toBeInTheDocument()
  })
})
