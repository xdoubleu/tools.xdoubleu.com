import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import ExportModal from '@/components/recipes/ExportModal'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

jest.mock('@/hooks/useMealPlans', () => ({
  useMealPlans: () => ({
    data: {
      plans: [
        { id: 'plan-1', name: 'Week Plan' },
        { id: 'plan-2', name: 'Party Plan' }
      ]
    },
    isLoading: false
  })
}))

jest.mock('@/hooks/useShoppingList', () => ({
  useMealPlanExportItems: (planId: string) => ({
    data: planId
      ? {
          dayItems: [
            {
              date: '2026-05-28',
              items: [{ name: 'garlic', amount: '2', unit: 'cloves' }]
            }
          ]
        }
      : undefined,
    isLoading: false
  })
}))

const customItems: ShoppingItem[] = [{ id: 'c1', amount: '1', unit: 'L', name: 'milk' }]

describe('ExportModal', () => {
  it('renders export buttons', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.getByRole('button', { name: /Copy to Clipboard/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Share to Apple Notes/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Download .txt/ })).toBeInTheDocument()
  })

  it('renders plan selector with options', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.getByRole('option', { name: 'Week Plan' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Party Plan' })).toBeInTheDocument()
  })

  it('shows per-day meal plan items when a plan is selected', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    const select = screen.getByRole('combobox')
    fireEvent.change(select, { target: { value: 'plan-1' } })
    expect(screen.getByText('2026-05-28')).toBeInTheDocument()
    expect(screen.getByText(/2 cloves — garlic/)).toBeInTheDocument()
  })

  it('does not show meal plan section when no plan is selected', () => {
    render(<ExportModal customItems={customItems} onClose={jest.fn()} />)
    expect(screen.queryByText('2026-05-28')).not.toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', () => {
    const onClose = jest.fn()
    render(<ExportModal customItems={customItems} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: /Close/ }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when backdrop is clicked', () => {
    const onClose = jest.fn()
    const { container } = render(<ExportModal customItems={customItems} onClose={onClose} />)
    const backdrop = container.firstChild as HTMLElement
    fireEvent.click(backdrop)
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})
