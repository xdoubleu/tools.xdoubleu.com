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
  useShoppingList
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
  it('uses /recipes/plans/:id as key when id is given', () => {
    renderHook(() => useMealPlan('plan-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes/plans/plan-1', expect.any(Function))
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
