import React from 'react'
import { render, screen, within, fireEvent, waitFor } from '@testing-library/react'

vi.mock('@/hooks/useRecipes', () => ({
  useRecipe: vi.fn(),
  useDeleteRecipe: vi.fn()
}))

vi.mock('@/hooks/useWakeLock', () => ({
  useWakeLock: vi.fn()
}))

vi.mock('@/hooks/navigation', () => ({
  useRouter: vi.fn()
}))

import RecipeClient from '@/app/recipes/[id]/RecipeClient'
import { useRecipe, useDeleteRecipe } from '@/hooks/useRecipes'
import { useWakeLock } from '@/hooks/useWakeLock'
import { useRouter } from '@/hooks/navigation'
import { WriteRejectedError } from '@/lib/offline/outbox'
import { create } from '@bufbuild/protobuf'
import {
  RecipeSchema,
  IngredientSchema,
  GetRecipeResponseSchema,
  ScaledIngredientSchema
} from '@/lib/gen/recipes/v1/recipes_pb'

const mockRouter = { push: vi.fn() }

const mockToggleCookingMode = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
  // @ts-expect-error -- partial mock
  vi.mocked(useRouter).mockReturnValue(mockRouter)
  vi.mocked(useDeleteRecipe).mockReturnValue(vi.fn())
  vi.mocked(useWakeLock).mockReturnValue({
    isActive: false,
    isSupported: true,
    toggle: mockToggleCookingMode
  })
})

describe('RecipeClient', () => {
  it('shows loading state', () => {
    vi.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: true,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })
    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Loading recipe…')).toBeInTheDocument()
  })

  it('shows error state', () => {
    vi.mocked(useRecipe).mockReturnValue({
      data: undefined,
      isLoading: false,
      isValidating: false,
      error: new Error('fail'),
      mutate: vi.fn()
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
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
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
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
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
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
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
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
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
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
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
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, {
        recipe,
        isOwner: false,
        scaledIngredients: scaled
      }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Vegetables')).toBeInTheDocument()
    expect(screen.getByText('Meat')).toBeInTheDocument()
  })

  it('shows a Cooking Mode toggle when the Wake Lock API is supported', async () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    const button = screen.getByRole('button', { name: 'Cooking Mode' })
    button.click()
    expect(mockToggleCookingMode).toHaveBeenCalled()
  })

  it('shows the active label once cooking mode is on', () => {
    vi.mocked(useWakeLock).mockReturnValue({
      isActive: true,
      isSupported: true,
      toggle: mockToggleCookingMode
    })
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByRole('button', { name: 'Cooking Mode: On' })).toBeInTheDocument()
  })

  it('hides the Cooking Mode toggle when the Wake Lock API is unsupported', () => {
    vi.mocked(useWakeLock).mockReturnValue({
      isActive: false,
      isSupported: false,
      toggle: mockToggleCookingMode
    })
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.queryByRole('button', { name: /Cooking Mode/ })).not.toBeInTheDocument()
  })

  it('shows a Draft badge for a draft recipe', () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2, isDraft: true })
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByText('Draft')).toBeInTheDocument()
  })

  it('omits the Draft badge for a proven recipe', () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.queryByText('Draft')).not.toBeInTheDocument()
  })

  it('lets an owner edit and confirm or cancel deletion from the header', async () => {
    const deleteRecipe = vi.fn().mockResolvedValue(undefined)
    vi.mocked(useDeleteRecipe).mockReturnValue(deleteRecipe)
    const recipe = create(RecipeSchema, {
      id: 'r1',
      name: 'Pasta',
      baseServings: 2,
      batchServings: 10
    })
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, {
        recipe,
        isOwner: true,
        canEdit: true,
        scaledIngredients: []
      }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    expect(screen.getByRole('heading', { level: 1, name: 'Pasta' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Edit' })).toHaveAttribute('href', '/recipes/r1/edit')
    expect(screen.getByText('Batch prep: 10 servings')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(deleteRecipe).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete Pasta?' })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(deleteRecipe).toHaveBeenCalledWith({ id: 'r1' }))
    await waitFor(() => expect(mockRouter.push).toHaveBeenCalledWith('/recipes/list'))
  })

  it('scales servings from the numeric servings field', () => {
    const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
    vi.mocked(useRecipe).mockReturnValue({
      data: create(GetRecipeResponseSchema, { recipe, isOwner: false, scaledIngredients: [] }),
      isLoading: false,
      isValidating: false,
      error: undefined,
      mutate: vi.fn()
    })

    render(<RecipeClient id="r1" />)
    const field = screen.getByRole('spinbutton', { name: 'Servings' })
    expect(field).toHaveAttribute('inputMode', 'numeric')
    fireEvent.change(field, { target: { value: '4' } })
    expect(useRecipe).toHaveBeenLastCalledWith('r1', 4)
  })

  describe('when the delete is rejected', () => {
    const confirmDelete = () => {
      fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
      const dialog = screen.getByRole('dialog', { name: 'Delete Pasta?' })
      fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    }

    const renderOwned = (deleteRecipe: () => Promise<void>) => {
      vi.mocked(useDeleteRecipe).mockReturnValue(deleteRecipe)
      const recipe = create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })
      vi.mocked(useRecipe).mockReturnValue({
        data: create(GetRecipeResponseSchema, {
          recipe,
          isOwner: true,
          canEdit: true,
          scaledIngredients: []
        }),
        isLoading: false,
        isValidating: false,
        error: undefined,
        mutate: vi.fn()
      })
      render(<RecipeClient id="r1" />)
      confirmDelete()
    }

    it("stays on the recipe and shows the server's reason", async () => {
      const reason = 'This recipe is used in a meal plan; remove it there first.'
      renderOwned(vi.fn().mockRejectedValue(new WriteRejectedError(reason)))

      expect(await screen.findByRole('alert')).toHaveTextContent(reason)
      expect(mockRouter.push).not.toHaveBeenCalled()
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    })

    it('falls back to a generic message for an unexpected failure', async () => {
      renderOwned(vi.fn().mockRejectedValue(new Error('IndexedDB is unavailable')))

      expect(await screen.findByRole('alert')).toHaveTextContent('Couldn’t delete this recipe.')
      expect(mockRouter.push).not.toHaveBeenCalled()
    })

    it('clears the message when the delete is retried', async () => {
      const deleteRecipe = vi
        .fn()
        .mockRejectedValueOnce(new WriteRejectedError('On a meal plan.'))
        .mockResolvedValue(undefined)
      renderOwned(deleteRecipe)
      expect(await screen.findByRole('alert')).toHaveTextContent('On a meal plan.')

      confirmDelete()
      await waitFor(() => expect(mockRouter.push).toHaveBeenCalledWith('/recipes/list'))
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })
  })
})
