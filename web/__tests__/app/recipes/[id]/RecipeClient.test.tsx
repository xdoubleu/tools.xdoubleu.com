import React from 'react'
import { render, screen, within, fireEvent, waitFor } from '@testing-library/react'

jest.mock('@/hooks/useRecipes', () => ({
  useRecipe: jest.fn(),
  useDeleteRecipe: jest.fn()
}))

jest.mock('@/hooks/useWakeLock', () => ({
  useWakeLock: jest.fn()
}))

jest.mock('next/navigation', () => ({
  useRouter: jest.fn()
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

import RecipeClient from '@/app/recipes/[id]/RecipeClient'
import { useRecipe, useDeleteRecipe } from '@/hooks/useRecipes'
import { useWakeLock } from '@/hooks/useWakeLock'
import { useRouter } from 'next/navigation'
import { create } from '@bufbuild/protobuf'
import {
  RecipeSchema,
  IngredientSchema,
  GetRecipeResponseSchema,
  ScaledIngredientSchema
} from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: jest.fn() }

const mockToggleCookingMode = jest.fn()

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- partial mock
  jest.mocked(useRouter).mockReturnValue(mockRouter)
  jest.mocked(useDeleteRecipe).mockReturnValue(jest.fn())
  jest.mocked(useWakeLock).mockReturnValue({
    isActive: false,
    isSupported: true,
    toggle: mockToggleCookingMode
  })
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
    expect(screen.getByText('Loading recipe…')).toBeInTheDocument()
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
    expect(screen.getAllByText('Vegetables')).toHaveLength(1)
    expect(screen.getAllByText('Meat')).toHaveLength(1)
    expect(screen.getByText('Onion')).toBeInTheDocument()
    expect(screen.getByText('Carrot')).toBeInTheDocument()
    expect(screen.getByText('Beef')).toBeInTheDocument()
  })

  it('visually contains grouped ingredients separately from ungrouped ones listed after', () => {
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
          groupName: 'Sauce'
        }),
        create(IngredientSchema, {
          id: 'i2',
          name: 'Salt',
          amount: 1,
          unit: 'tsp',
          sortOrder: 2
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
    const heading = screen.getByRole('heading', { name: 'Sauce', level: 3 })
    const groupContainer = heading.parentElement
    if (!groupContainer) throw new Error('expected group container')
    expect(within(groupContainer).getByText('Onion')).toBeInTheDocument()
    expect(within(groupContainer).queryByText('Salt')).not.toBeInTheDocument()
    expect(screen.getByText('Salt')).toBeInTheDocument()
  })

  it('wraps ungrouped ingredients in a card matching the grouped sections', () => {
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
          groupName: 'Sauce'
        }),
        create(IngredientSchema, {
          id: 'i2',
          name: 'Salt',
          amount: 1,
          unit: 'tsp',
          sortOrder: 2
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
    // Ungrouped ingredients use the same card shell, without a header.
    const saltCard = screen.getByText('Salt').closest('.rounded-2xl')
    if (!(saltCard instanceof HTMLElement)) throw new Error('expected ungrouped card')
    expect(saltCard).toHaveClass('border')
    expect(within(saltCard).queryByRole('heading')).not.toBeInTheDocument()
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

  it('shows a Cooking Mode toggle when the Wake Lock API is supported', async () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    const button = screen.getByRole('button', { name: 'Cooking Mode' })
    button.click()
    expect(mockToggleCookingMode).toHaveBeenCalled()
  })

  it('shows the active label once cooking mode is on', () => {
    jest.mocked(useWakeLock).mockReturnValue({
      isActive: true,
      isSupported: true,
      toggle: mockToggleCookingMode
    })
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByRole('button', { name: 'Cooking Mode: On' })).toBeInTheDocument()
  })

  it('hides the Cooking Mode toggle when the Wake Lock API is unsupported', () => {
    jest.mocked(useWakeLock).mockReturnValue({
      isActive: false,
      isSupported: false,
      toggle: mockToggleCookingMode
    })
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.queryByRole('button', { name: /Cooking Mode/ })).not.toBeInTheDocument()
  })

  it('shows a Draft badge for a draft recipe', () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2, isDraft: true })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Draft')).toBeInTheDocument()
  })

  it('omits the Draft badge for a proven recipe', () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.queryByText('Draft')).not.toBeInTheDocument()
  })

  it('lets an owner edit and confirm or cancel deletion from the header', async () => {
    const deleteRecipe = jest.fn().mockResolvedValue(undefined)
    jest.mocked(useDeleteRecipe).mockReturnValue(deleteRecipe)
    const recipe = create(RecipeSchema, {
      id: 'r1',
      name: 'Pasta',
      baseServings: 2,
      batchServings: 10
    })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, {
        recipe,
        isOwner: true,
        canEdit: true,
        scaledIngredients: []
      }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByRole('heading', { level: 1, name: 'Pasta' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Edit' })).toHaveAttribute('href', '/recipes/r1/edit')
    expect(screen.getByText('Batch prep: 10 servings')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('button', { name: 'Confirm delete' })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(screen.getByRole('button', { name: 'Confirm delete' }))
    await waitFor(() => expect(mockRouter.push).toHaveBeenCalledWith('/recipes/list'))
  })

  it('scales servings from the numeric servings field', () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    jest.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: jest.fn()
    })

    render(<RecipeClient id="r1" />)
    const field = screen.getByRole('spinbutton', { name: 'Servings' })
    expect(field).toHaveAttribute('inputMode', 'numeric')
    fireEvent.change(field, { target: { value: '4' } })
    expect(useRecipe).toHaveBeenLastCalledWith('r1', 4)
  })
})
