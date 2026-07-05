import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useRecipes', () => ({
  useRecipes: jest.fn(),
  useRecipeBookShares: jest.fn(() => ({ data: undefined, mutate: jest.fn() })),
  useShareRecipeBook: jest.fn(() => jest.fn()),
  useUnshareRecipeBook: jest.fn(() => jest.fn())
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/recipes/ShareModal', () => ({
  __esModule: true,
  default: () => <div data-testid="share-modal" />
}))

import RecipesListClient from '@/components/recipes/RecipesListClient'
import { useRecipes } from '@/hooks/useRecipes'
import { create } from '@bufbuild/protobuf'
import { RecipeSchema, ListRecipesResponseSchema } from '@/lib/gen/recipes/v1/recipes_pb'
import type { ListRecipesResponse } from '@/lib/gen/recipes/v1/recipes_pb'

function mockRecipes(value: { data?: ListRecipesResponse; error?: Error; isLoading: boolean }) {
  jest.mocked(useRecipes).mockReturnValue({
    data: value.data,
    error: value.error,
    isLoading: value.isLoading,
    isValidating: false,
    mutate: jest.fn(async () => undefined)
  })
}

beforeEach(() => jest.clearAllMocks())

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
  })
})
