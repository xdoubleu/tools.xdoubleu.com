import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
jest.mock('@/lib/gen/recipes/v1/recipes_connect', () => ({
  RecipesService: {}
}))

import useSWR from 'swr'
import {
  useRecipes,
  useRecipe,
  useCreateRecipe,
  useUpdateRecipe,
  useDeleteRecipe,
  useShareRecipe,
  useUnshareRecipe
} from '@/hooks/useRecipes'

const mockUseSWR = useSWR as jest.Mock

beforeEach(() => {
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
  it('uses /recipes/:id as key when id is given', () => {
    renderHook(() => useRecipe('r-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/r-1', expect.any(Function))
  })

  it('includes servings in key when provided', () => {
    renderHook(() => useRecipe('r-1', 4))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/r-1?servings=4', expect.any(Function))
  })

  it('passes null as key when id is empty', () => {
    renderHook(() => useRecipe(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
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

  it('useShareRecipe returns a function', () => {
    const { result } = renderHook(() => useShareRecipe())
    expect(typeof result.current).toBe('function')
  })

  it('useUnshareRecipe returns a function', () => {
    const { result } = renderHook(() => useUnshareRecipe())
    expect(typeof result.current).toBe('function')
  })
})
