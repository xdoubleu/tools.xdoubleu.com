import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
jest.mock('@/lib/gen/recipes/v1/recipes_connect', () => ({
  RecipesService: {}
}))
jest.mock('@/lib/gen/recipes/v1/mealplans_connect', () => ({
  MealPlansService: {}
}))

import useSWR from 'swr'
import {
  useRecipes,
  useRecipe,
  useMealPlans,
  useMealPlan,
  useShoppingList,
  useCreateRecipe,
  useUpdateRecipe,
  useDeleteRecipe,
  useShareRecipe,
  useUnshareRecipe,
  useCreatePlan,
  useUpdatePlan,
  useDeletePlan,
  useAddMeal,
  useDeleteMeal,
  useSharePlan,
  useUnsharePlan
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

describe('useMealPlans', () => {
  it('uses /recipes/plans as key', () => {
    renderHook(() => useMealPlans())
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/plans', expect.any(Function))
  })
})

describe('useMealPlan', () => {
  it('uses /recipes/plans/:id?offset=0 as key by default', () => {
    renderHook(() => useMealPlan('plan-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/plans/plan-1?offset=0', expect.any(Function))
  })

  it('includes offset in key when non-zero', () => {
    renderHook(() => useMealPlan('plan-1', 2))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/plans/plan-1?offset=2', expect.any(Function))
  })

  it('passes null as key when id is empty', () => {
    renderHook(() => useMealPlan(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useShoppingList', () => {
  it('uses shopping list path when planId is given', () => {
    renderHook(() => useShoppingList('plan-2'))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/plans/plan-2/shopping', expect.any(Function))
  })

  it('passes null as key when planId is empty', () => {
    renderHook(() => useShoppingList(''))
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

  it('useCreatePlan returns a function', () => {
    const { result } = renderHook(() => useCreatePlan())
    expect(typeof result.current).toBe('function')
  })

  it('useUpdatePlan returns a function', () => {
    const { result } = renderHook(() => useUpdatePlan())
    expect(typeof result.current).toBe('function')
  })

  it('useDeletePlan returns a function', () => {
    const { result } = renderHook(() => useDeletePlan())
    expect(typeof result.current).toBe('function')
  })

  it('useAddMeal returns a function', () => {
    const { result } = renderHook(() => useAddMeal())
    expect(typeof result.current).toBe('function')
  })

  it('useDeleteMeal returns a function', () => {
    const { result } = renderHook(() => useDeleteMeal())
    expect(typeof result.current).toBe('function')
  })

  it('useSharePlan returns a function', () => {
    const { result } = renderHook(() => useSharePlan())
    expect(typeof result.current).toBe('function')
  })

  it('useUnsharePlan returns a function', () => {
    const { result } = renderHook(() => useUnsharePlan())
    expect(typeof result.current).toBe('function')
  })
})
