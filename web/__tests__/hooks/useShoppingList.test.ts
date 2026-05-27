import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
jest.mock('@/lib/gen/shoppinglist/v1/shoppinglist_pb', () => ({
  ShoppingListService: {}
}))

import useSWR from 'swr'
import { useShoppingList } from '@/hooks/useShoppingList'

const mockUseSWR = useSWR as jest.Mock

beforeEach(() => {
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useShoppingList', () => {
  it('uses /shoppinglist/:planId as key when planId is given', () => {
    renderHook(() => useShoppingList('plan-2'))
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/plan-2', expect.any(Function))
  })

  it('passes null as key when planId is empty', () => {
    renderHook(() => useShoppingList(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})
