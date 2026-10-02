import React from 'react'
import { render, screen } from '@testing-library/react'

const push = jest.fn()
let mockOnSave: (id: string, synced: boolean) => void = () => {}
let mockOnCancel: () => void = () => {}

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push })
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/recipes/RecipeForm', () => ({
  __esModule: true,
  default: ({
    onSave,
    onCancel
  }: {
    onSave: (id: string, synced: boolean) => void
    onCancel: () => void
  }) => {
    mockOnSave = onSave
    mockOnCancel = onCancel
    return <div data-testid="recipe-form" />
  }
}))

import NewRecipePage from '@/app/recipes/new/page'

describe('NewRecipePage', () => {
  it('renders the heading and form', () => {
    render(<NewRecipePage />)
    expect(screen.getByRole('heading', { name: 'New Recipe' })).toBeInTheDocument()
    expect(screen.getByTestId('recipe-form')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Recipes' })).toHaveAttribute('href', '/recipes/list')
    expect(screen.getByText('New')).toBeInTheDocument()
  })

  it('opens the saved recipe once it synced', () => {
    render(<NewRecipePage />)
    mockOnSave('r-1', true)
    expect(push).toHaveBeenCalledWith('/recipes/r-1')
  })

  it('returns to the list while the recipe waits to sync', () => {
    render(<NewRecipePage />)
    mockOnSave('r-1', false)
    expect(push).toHaveBeenCalledWith('/recipes/list')
  })

  it('returns to the list on cancel', () => {
    render(<NewRecipePage />)
    mockOnCancel()
    expect(push).toHaveBeenCalledWith('/recipes/list')
  })
})
