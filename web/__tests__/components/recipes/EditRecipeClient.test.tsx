import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useRecipes', () => ({
  useRecipe: jest.fn()
}))

jest.mock('next/navigation', () => ({
  useRouter: jest.fn()
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/recipes/RecipeForm', () => {
  return function MockRecipeForm() {
    return <div data-testid="recipe-form">recipe-form-mock</div>
  }
})

import EditRecipeClient from '@/app/recipes/[id]/edit/EditRecipeClient'
import { useRecipe } from '@/hooks/useRecipes'
import { useRouter } from 'next/navigation'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: jest.fn() }
const mockRecipe = {
  id: 'recipe-1',
  name: 'Pasta Carbonara'
} as unknown as Recipe

beforeEach(() => {
  jest.clearAllMocks()
  ;(useRouter as jest.Mock).mockReturnValue(mockRouter)
})

describe('EditRecipeClient', () => {
  it('shows loading state when isLoading is true', () => {
    ;(useRecipe as jest.Mock).mockReturnValue({
      data: null,
      isLoading: true,
      error: null
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByText('Loading recipe...')).toBeInTheDocument()
  })

  it('shows error state when error is present', () => {
    ;(useRecipe as jest.Mock).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Failed to fetch')
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByText('Failed to load recipe.')).toBeInTheDocument()
  })

  it('renders RecipeForm when recipe is loaded', () => {
    ;(useRecipe as jest.Mock).mockReturnValue({
      data: { recipe: mockRecipe },
      isLoading: false,
      error: null
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByTestId('recipe-form')).toBeInTheDocument()
  })

  it('calls useRecipe with the provided id', () => {
    ;(useRecipe as jest.Mock).mockReturnValue({
      data: null,
      isLoading: false,
      error: null
    })

    render(<EditRecipeClient id="recipe-123" />)
    expect(useRecipe).toHaveBeenCalledWith('recipe-123')
  })

  it('renders back link with correct href', () => {
    ;(useRecipe as jest.Mock).mockReturnValue({
      data: null,
      isLoading: false,
      error: null
    })

    render(<EditRecipeClient id="recipe-1" />)
    const backLink = screen.getByText(/Back to Recipe/).closest('a')
    expect(backLink).toHaveAttribute('href', '/recipes/recipe-1')
  })

  it('renders page title', () => {
    ;(useRecipe as jest.Mock).mockReturnValue({
      data: null,
      isLoading: false,
      error: null
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByText('Edit Recipe')).toBeInTheDocument()
  })
})
