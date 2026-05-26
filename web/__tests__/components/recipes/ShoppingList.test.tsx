import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ShoppingList from '@/components/recipes/ShoppingList'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

describe('ShoppingList', () => {
  const mockItems: ShoppingItem[] = [
    { amount: '2', unit: 'cups', name: 'flour' },
    { amount: '1', unit: 'tbsp', name: 'sugar' }
  ]

  const mixedItems: ShoppingItem[] = [
    { amount: '2', unit: 'cups', name: 'flour' },
    { id: 'custom-1', amount: '1', unit: 'L', name: 'milk' }
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

  describe('delete button', () => {
    it('does not render delete buttons when onDelete is not provided', () => {
      render(<ShoppingList items={mixedItems} />)
      expect(screen.queryByRole('button', { name: /Remove/ })).not.toBeInTheDocument()
    })

    it('does not render delete button for recipe-derived items (no id)', () => {
      const onDelete = jest.fn()
      render(<ShoppingList items={mixedItems} onDelete={onDelete} />)
      expect(screen.queryByRole('button', { name: /Remove flour/ })).not.toBeInTheDocument()
    })

    it('renders delete button only for custom items', () => {
      const onDelete = jest.fn()
      render(<ShoppingList items={mixedItems} onDelete={onDelete} />)
      expect(screen.getByRole('button', { name: /Remove milk/ })).toBeInTheDocument()
    })

    it('calls onDelete with the item id when delete button is clicked', async () => {
      const onDelete = jest.fn().mockResolvedValue(undefined)
      render(<ShoppingList items={mixedItems} onDelete={onDelete} />)
      fireEvent.click(screen.getByRole('button', { name: /Remove milk/ }))
      await waitFor(() => expect(onDelete).toHaveBeenCalledWith('custom-1'))
    })
  })

  it('renders export buttons', () => {
    render(<ShoppingList items={mockItems} />)
    expect(screen.getByRole('button', { name: /Copy to Clipboard/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Share to Apple Notes/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Download .txt/ })).toBeInTheDocument()
  })

  describe('Share to Apple Notes', () => {
    it('uses Web Share API when available', async () => {
      const mockShare = jest.fn().mockResolvedValue(undefined)
      Object.defineProperty(navigator, 'share', { value: mockShare, configurable: true })

      render(<ShoppingList items={mockItems} />)
      fireEvent.click(screen.getByRole('button', { name: /Share to Apple Notes/ }))

      await waitFor(() => {
        expect(mockShare).toHaveBeenCalledWith(
          expect.objectContaining({ text: expect.stringContaining('flour') })
        )
      })
    })

    it('falls back to clipboard when Web Share API is unavailable', async () => {
      Object.defineProperty(navigator, 'share', { value: undefined, configurable: true })
      const mockWriteText = jest.fn().mockResolvedValue(undefined)
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText: mockWriteText },
        configurable: true
      })

      render(<ShoppingList items={mockItems} />)
      fireEvent.click(screen.getByRole('button', { name: /Share to Apple Notes/ }))

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalled()
        expect(screen.getByText('Copied (Apple Notes format)!')).toBeInTheDocument()
      })
    })
  })

  it('copies to clipboard and shows feedback', async () => {
    const mockWriteText = jest.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: mockWriteText },
      configurable: true
    })

    render(<ShoppingList items={mockItems} />)
    fireEvent.click(screen.getByRole('button', { name: /Copy to Clipboard/ }))

    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalled()
      expect(screen.getByText('Copied!')).toBeInTheDocument()
    })
  })
})
