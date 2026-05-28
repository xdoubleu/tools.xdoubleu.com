import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
jest.mock('@/lib/gen/shoppinglist/v1/shoppinglist_pb', () => ({
  ShoppingListService: {}
}))

import useSWR from 'swr'
import { useCustomList, useMealPlanExportItems } from '@/hooks/useShoppingList'

const mockUseSWR = useSWR as jest.Mock

beforeEach(() => {
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useCustomList', () => {
  it('uses /shoppinglist as the SWR key', () => {
    renderHook(() => useCustomList())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist', expect.any(Function))
  })
})

describe('useMealPlanExportItems', () => {
  it('uses /shoppinglist/export/:planId as key when planId is given', () => {
    renderHook(() => useMealPlanExportItems('plan-2'))
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/export/plan-2', expect.any(Function))
  })

  it('passes null as key when planId is empty', () => {
    renderHook(() => useMealPlanExportItems(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})
