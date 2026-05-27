import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
jest.mock('@/lib/gen/mealplans/v1/mealplans_pb', () => ({
  MealPlansService: {}
}))

import useSWR from 'swr'
import {
  useMealPlans,
  useMealPlan,
  useCreatePlan,
  useUpdatePlan,
  useDeletePlan,
  useAddMeal,
  useDeleteMeal,
  useMoveMeal,
  useSharePlan,
  useUnsharePlan
} from '@/hooks/useMealPlans'

const mockUseSWR = useSWR as jest.Mock

beforeEach(() => {
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useMealPlans', () => {
  it('uses /mealplans as key', () => {
    renderHook(() => useMealPlans())
    expect(mockUseSWR).toHaveBeenCalledWith('/mealplans', expect.any(Function))
  })
})

describe('useMealPlan', () => {
  it('uses /mealplans/:id?offset=0 as key by default', () => {
    renderHook(() => useMealPlan('plan-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/mealplans/plan-1?offset=0', expect.any(Function))
  })

  it('includes offset in key when non-zero', () => {
    renderHook(() => useMealPlan('plan-1', 2))
    expect(mockUseSWR).toHaveBeenCalledWith('/mealplans/plan-1?offset=2', expect.any(Function))
  })

  it('passes null as key when id is empty', () => {
    renderHook(() => useMealPlan(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('mutation hooks return functions', () => {
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

  it('useMoveMeal returns a function', () => {
    const { result } = renderHook(() => useMoveMeal())
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
