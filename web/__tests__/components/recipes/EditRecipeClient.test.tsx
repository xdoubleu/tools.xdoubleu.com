import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/hooks/useRecipes', () => ({
  useRecipe: vi.fn()
}))

vi.mock('@/hooks/navigation', () => ({
  useRouter: vi.fn()
}))

vi.mock('@/components/recipes/RecipeForm', () => {
  return {
    default: function MockRecipeForm() {
      return <div data-testid="recipe-form">recipe-form-mock</div>
    }
  }
})

import EditRecipeClient from '@/app/recipes/[id]/edit/EditRecipeClient'
import { useRecipe } from '@/hooks/useRecipes'
import { useRouter } from '@/hooks/navigation'
import { create } from '@bufbuild/protobuf'
import { RecipeSchema, GetRecipeResponseSchema } from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: vi.fn() }
const mockRecipe = create(RecipeSchema, { id: 'recipe-1', name: 'Pasta Carbonara' })

beforeEach(() => {
  vi.clearAllMocks()
  // @ts-expect-error -- mock router returns partial AppRouterInstance
  vi.mocked(useRouter).mockReturnValue(mockRouter)
})

describe('EditRecipeClient', () => {
  it('shows loading state when isLoading is true', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByText('Loading recipe…')).toBeInTheDocument()
  })

  it('shows error state when error is present', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to fetch')
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByText('Failed to load recipe.')).toBeInTheDocument()
  })

  it('renders RecipeForm when recipe is loaded', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe: mockRecipe, isOwner: true }),
      isLoading: false,
      error: undefined
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByTestId('recipe-form')).toBeInTheDocument()
  })

  it('calls useRecipe with the provided id', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<EditRecipeClient id="recipe-123" />)
    expect(useRecipe).toHaveBeenCalledWith('recipe-123')
  })

  it('renders back link with correct href', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<EditRecipeClient id="recipe-1" />)
    const backLink = screen.getByRole('link', { name: 'Recipe' })
    expect(backLink).toHaveAttribute('href', '/recipes/recipe-1')
  })

  it('renders page title', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<EditRecipeClient id="recipe-1" />)
    expect(screen.getByText('Edit recipe')).toBeInTheDocument()
  })
})
