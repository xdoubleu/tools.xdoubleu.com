import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useRecipes', () => ({
  useRecipe: jest.fn(),
  useDeleteRecipe: jest.fn(),
  useShareRecipe: jest.fn(),
  useUnshareRecipe: jest.fn()
}))

jest.mock('next/navigation', () => ({
  useRouter: jest.fn()
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/recipes/ShareModal', () => {
  return function MockShareModal() {
    return <div data-testid="share-modal" />
  }
})

import RecipeClient from '@/app/recipes/[id]/RecipeClient'
import { useRecipe, useDeleteRecipe, useShareRecipe, useUnshareRecipe } from '@/hooks/useRecipes'
import { useRouter } from 'next/navigation'
import { create } from '@bufbuild/protobuf'
import {
  RecipeSchema,
  IngredientSchema,
  GetRecipeResponseSchema,
  ScaledIngredientSchema
} from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: jest.fn() }

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- partial mock
  jest.mocked(useRouter).mockReturnValue(mockRouter)
  jest.mocked(useDeleteRecipe).mockReturnValue(jest.fn())
  jest.mocked(useShareRecipe).mockReturnValue(jest.fn())
  jest.mocked(useUnshareRecipe).mockReturnValue(jest.fn())
})

describe('RecipeClient', () => {
  it('shows loading state', () => {
    jest.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: true,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })
    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Loading recipe...')).toBeInTheDocument()
  })

  it('shows error state', () => {
    jest.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: false,
      isValidating: false,
      error: new Error('fail'),
      mutate: jest.fn()
    })
    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Failed to load recipe.')).toBeInTheDocument()
  })

  it('renders ingredients without groups as a flat list', () => {
    const recipe = create(RecipeSchema, {
      id: 'r1',
      name: 'Pasta',
      baseServings: 2,
      ingredients: [
        create(IngredientSchema, { id: 'i1', name: 'Flour', amount: 200, unit: 'g', sortOrder: 1 }),
        create(IngredientSchema, { id: 'i2', name: 'Egg', amount: 2, unit: '', sortOrder: 2 })
      ]
    })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Flour')).toBeInTheDocument()
    expect(screen.getByText('Egg')).toBeInTheDocument()
    expect(screen.queryByRole('paragraph')).not.toBeInTheDocument()
  })

  it('renders group headers when ingredients have group names', () => {
    const recipe = create(RecipeSchema, {
      id: 'r1',
      name: 'Stew',
      baseServings: 4,
      ingredients: [
        create(IngredientSchema, {
          id: 'i1',
          name: 'Onion',
          amount: 1,
          unit: '',
          sortOrder: 1,
          groupName: 'Vegetables'
        }),
        create(IngredientSchema, {
          id: 'i2',
          name: 'Carrot',
          amount: 2,
          unit: '',
          sortOrder: 2,
          groupName: 'Vegetables'
        }),
        create(IngredientSchema, {
          id: 'i3',
          name: 'Beef',
          amount: 500,
          unit: 'g',
          sortOrder: 3,
          groupName: 'Meat'
        })
      ]
    })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Vegetables')).toBeInTheDocument()
    expect(screen.getByText('Meat')).toBeInTheDocument()
    expect(screen.getByText('Onion')).toBeInTheDocument()
    expect(screen.getByText('Beef')).toBeInTheDocument()
  })

  it('groups non-consecutive ingredients with the same group name together', () => {
    const recipe = create(RecipeSchema, {
      id: 'r1',
      name: 'Stew',
      baseServings: 4,
      ingredients: [
        create(IngredientSchema, {
          id: 'i1',
          name: 'Onion',
          amount: 1,
          unit: '',
          sortOrder: 1,
          groupName: 'Vegetables'
        }),
        create(IngredientSchema, {
          id: 'i2',
          name: 'Beef',
          amount: 500,
          unit: 'g',
          sortOrder: 2,
          groupName: 'Meat'
        }),
        create(IngredientSchema, {
          id: 'i3',
          name: 'Carrot',
          amount: 2,
          unit: '',
          sortOrder: 3,
          groupName: 'Vegetables'
        })
      ]
    })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    // 'Vegetables' header should appear exactly once even though its ingredients are non-consecutive
    expect(screen.getAllByText('Vegetables')).toHaveLength(1)
    expect(screen.getAllByText('Meat')).toHaveLength(1)
    expect(screen.getByText('Onion')).toBeInTheDocument()
    expect(screen.getByText('Carrot')).toBeInTheDocument()
    expect(screen.getByText('Beef')).toBeInTheDocument()
  })

  it('shows group headers with scaled ingredients by mapping from sorted originals', () => {
    const recipe = create(RecipeSchema, {
      id: 'r1',
      name: 'Stew',
      baseServings: 4,
      ingredients: [
        create(IngredientSchema, {
          id: 'i1',
          name: 'Onion',
          amount: 1,
          unit: '',
          sortOrder: 1,
          groupName: 'Vegetables'
        }),
        create(IngredientSchema, {
          id: 'i2',
          name: 'Beef',
          amount: 500,
          unit: 'g',
          sortOrder: 2,
          groupName: 'Meat'
        })
      ]
    })
    const scaled = [
      create(ScaledIngredientSchema, { name: 'Onion', amount: '2', unit: '' }),
      create(ScaledIngredientSchema, { name: 'Beef', amount: '1000', unit: 'g' })
    ]
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, {
        recipe,
        isOwner: false,
        scaledIngredients: scaled
      }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Vegetables')).toBeInTheDocument()
    expect(screen.getByText('Meat')).toBeInTheDocument()
  })
})
