import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockClient = {
  listRecipeBookShares: jest.fn().mockResolvedValue({ shares: [] }),
  shareRecipeBook: jest.fn().mockResolvedValue({}),
  unshareRecipeBook: jest.fn().mockResolvedValue({}),
  listRecipes: jest.fn().mockResolvedValue({ recipes: [{ id: 'r-1' }], hasMore: true })
}
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/recipes/v1/recipes_pb', () => ({
  RecipesService: {}
}))

import useSWR from 'swr'
import {
  useRecipes,
  useRecipe,
  useCreateRecipe,
  useUpdateRecipe,
  useDeleteRecipe,
  useRecipeBookShares,
  useShareRecipeBook,
  useUnshareRecipeBook,
  useFetchRecipesPage
} from '@/hooks/useRecipes'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useRecipes', () => {
  it('uses /recipes as key', () => {
    renderHook(() => useRecipes())
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes', expect.any(Function))
  })
})

describe('useRecipe', () => {
  const opts = { keepPreviousData: true }

  it('uses /recipes/:id as key when id is given', () => {
    renderHook(() => useRecipe('r-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/r-1', expect.any(Function), opts)
  })

  it('includes servings in key when provided', () => {
    renderHook(() => useRecipe('r-1', 4))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/r-1?servings=4', expect.any(Function), opts)
  })

  it('passes null as key when id is empty', () => {
    renderHook(() => useRecipe(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function), opts)
  })
})

describe('mutation hooks return functions', () => {
  it('useCreateRecipe returns a function', () => {
    const { result } = renderHook(() => useCreateRecipe())
    expect(typeof result.current).toBe('function')
  })

  it('useUpdateRecipe returns a function', () => {
    const { result } = renderHook(() => useUpdateRecipe())
    expect(typeof result.current).toBe('function')
  })

  it('useDeleteRecipe returns a function', () => {
    const { result } = renderHook(() => useDeleteRecipe())
    expect(typeof result.current).toBe('function')
  })

  it('useShareRecipeBook calls shareRecipeBook with contact and permission', () => {
    const { result } = renderHook(() => useShareRecipeBook())
    result.current('u-1', true)
    expect(mockClient.shareRecipeBook).toHaveBeenCalledWith({
      contactUserId: 'u-1',
      canEdit: true
    })
  })

  it('useUnshareRecipeBook calls unshareRecipeBook with the target', () => {
    const { result } = renderHook(() => useUnshareRecipeBook())
    result.current('u-2')
    expect(mockClient.unshareRecipeBook).toHaveBeenCalledWith({ targetUserId: 'u-2' })
  })

  it('useRecipeBookShares uses /recipes/book-shares as key', () => {
    renderHook(() => useRecipeBookShares())
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/book-shares', expect.any(Function))
  })
})

describe('useFetchRecipesPage', () => {
  it('fetches a page at the given offset and maps the response', async () => {
    const { result } = renderHook(() => useFetchRecipesPage())
    const page = await result.current(50)
    expect(mockClient.listRecipes).toHaveBeenCalledWith({ limit: 50, offset: 50 })
    expect(page).toEqual({ items: [{ id: 'r-1' }], hasMore: true })
  })
})
