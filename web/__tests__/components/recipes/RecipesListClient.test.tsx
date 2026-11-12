import React from 'react'
import { fireEvent, render, screen } from '@testing-library/react'

vi.mock('@/hooks/useRecipes', () => ({
  useRecipes: vi.fn(),
  useFetchRecipesPage: vi.fn(() => vi.fn())
}))

import RecipesListClient from '@/components/recipes/RecipesListClient'
import { useRecipes } from '@/hooks/useRecipes'
import { create } from '@bufbuild/protobuf'
import { RecipeSchema, ListRecipesResponseSchema } from '@/lib/gen/recipes/v1/recipes_pb'
import type { ListRecipesResponse } from '@/lib/gen/recipes/v1/recipes_pb'

function mockRecipes(value: { data?: ListRecipesResponse; error?: Error; isLoading: boolean }) {
  vi.mocked(useRecipes).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: vi.fn(async () => undefined)
  })
}

beforeEach(() => vi.clearAllMocks())

describe('RecipesListClient', () => {
  it('shows a loading state', () => {
    mockRecipes({ isLoading: true })
    render(<RecipesListClient />)
    expect(screen.getByText('Loading recipes…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockRecipes({ error: new Error('boom'), isLoading: false })
    render(<RecipesListClient />)
    expect(screen.getByText('Failed to load recipes.')).toBeInTheDocument()
  })

  it('renders recipe cards', () => {
    mockRecipes({
      data: create(ListRecipesResponseSchema, {
        recipes: [create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 })]
      }),
      isLoading: false
    })
    render(<RecipesListClient />)
    expect(screen.getByRole('link', { name: /Pasta/ })).toHaveAttribute('href', '/recipes/r1')
    expect(screen.getByText('Serves 2')).toBeInTheDocument()
  })

  it('marks draft recipes with a badge and leaves proven ones unmarked', () => {
    mockRecipes({
      data: create(ListRecipesResponseSchema, {
        recipes: [
          create(RecipeSchema, { id: 'r1', name: 'Proven', baseServings: 2 }),
          create(RecipeSchema, { id: 'r2', name: 'Untested', baseServings: 2, isDraft: true })
        ]
      }),
      isLoading: false
    })
    render(<RecipesListClient />)
    expect(screen.getByRole('link', { name: /Untested/ })).toHaveTextContent('Draft')
    expect(screen.getByRole('link', { name: /Proven/ })).not.toHaveTextContent('Draft')
  })

  const three = () =>
    mockRecipes({
      data: create(ListRecipesResponseSchema, {
        recipes: [
          create(RecipeSchema, { id: 'r1', name: 'Pasta', baseServings: 2 }),
          create(RecipeSchema, { id: 'r2', name: 'Soup', baseServings: 4, isDraft: true }),
          create(RecipeSchema, { id: 'r3', name: 'Pancakes', baseServings: 3, isDraft: true })
        ]
      }),
      isLoading: false
    })

  it('filters by name as you type', () => {
    three()
    render(<RecipesListClient />)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search recipes' }), {
      target: { value: '  PA ' }
    })
    expect(screen.getByRole('link', { name: /Pasta/ })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Pancakes/ })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Soup/ })).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('2 results')
  })

  it('narrows to drafts and back', () => {
    three()
    render(<RecipesListClient />)
    const chip = screen.getByRole('button', { name: /Drafts/ })
    fireEvent.click(chip)
    expect(screen.queryByRole('link', { name: /Pasta/ })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Soup/ })).toBeInTheDocument()
    fireEvent.click(chip)
    expect(screen.getByRole('link', { name: /Pasta/ })).toBeInTheDocument()
  })

  it('says so when nothing matches the search', () => {
    three()
    render(<RecipesListClient />)
    fireEvent.change(screen.getByRole('searchbox', { name: 'Search recipes' }), {
      target: { value: 'zzz' }
    })
    expect(screen.getByText('No recipes match.')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Pasta/ })).not.toBeInTheDocument()
  })

  it('invites the first recipe when there are none', () => {
    mockRecipes({ data: create(ListRecipesResponseSchema, { recipes: [] }), isLoading: false })
    render(<RecipesListClient />)
    expect(screen.getByText('No recipes yet. Create your first one!')).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'New recipe' })).toHaveLength(2)
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
  })
})
