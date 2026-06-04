import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockClient = {
  getCustomList: jest.fn().mockResolvedValue({}),
  getMealPlanExportItems: jest.fn().mockResolvedValue({}),
  getPlanIngredientGroups: jest.fn().mockResolvedValue({}),
  listCategories: jest.fn().mockResolvedValue({}),
  listStores: jest.fn().mockResolvedValue({}),
  getStoreCategories: jest.fn().mockResolvedValue({}),
  listItemNames: jest.fn().mockResolvedValue({}),
  listItemCategories: jest.fn().mockResolvedValue({})
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/shoppinglist/v1/shoppinglist_pb', () => ({
  ShoppingListService: {}
}))

import useSWR from 'swr'
import {
  useCustomList,
  useMealPlanExportItems,
  usePlanIngredientGroups,
  useCategories,
  useStores,
  useStoreCategories,
  useItemNames,
  useItemCategories
} from '@/hooks/useShoppingList'

const mockUseSWR = jest.mocked(useSWR)

// Extract and invoke the SWR fetcher function captured in mock.calls.
// Using typeof guard to narrow without an unsafe type assertion.
async function callFetcher() {
  const fetcher = mockUseSWR.mock.calls[0]?.[1]
  if (typeof fetcher === 'function') await fetcher()
}

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useCustomList', () => {
  it('uses /shoppinglist as the SWR key', () => {
    renderHook(() => useCustomList())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist', expect.any(Function))
  })

  it('fetcher calls getCustomList', async () => {
    renderHook(() => useCustomList())
    await callFetcher()
    expect(mockClient.getCustomList).toHaveBeenCalledWith({})
  })
})

describe('useMealPlanExportItems', () => {
  it('uses /shoppinglist/export/:planId as key when planId is given', () => {
    renderHook(() => useMealPlanExportItems('plan-2'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/shoppinglist/export/plan-2?excluded=',
      expect.any(Function)
    )
  })

  it('encodes excluded groups in the SWR key', () => {
    renderHook(() => useMealPlanExportItems('plan-2', ['sauce', 'pasta']))
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/shoppinglist/export/plan-2?excluded=pasta,sauce',
      expect.any(Function)
    )
  })

  it('passes null as key when planId is empty', () => {
    renderHook(() => useMealPlanExportItems(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('usePlanIngredientGroups', () => {
  it('uses /shoppinglist/groups/:planId as key when planId is given', () => {
    renderHook(() => usePlanIngredientGroups('plan-3'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/shoppinglist/groups/plan-3',
      expect.any(Function)
    )
  })

  it('passes null as key when planId is empty', () => {
    renderHook(() => usePlanIngredientGroups(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useCategories', () => {
  it('uses /shoppinglist/categories as the SWR key', () => {
    renderHook(() => useCategories())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/categories', expect.any(Function))
  })

  it('fetcher calls listCategories', async () => {
    renderHook(() => useCategories())
    await callFetcher()
    expect(mockClient.listCategories).toHaveBeenCalledWith({})
  })
})

describe('useStores', () => {
  it('uses /shoppinglist/stores as the SWR key', () => {
    renderHook(() => useStores())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/stores', expect.any(Function))
  })

  it('fetcher calls listStores', async () => {
    renderHook(() => useStores())
    await callFetcher()
    expect(mockClient.listStores).toHaveBeenCalledWith({})
  })
})

describe('useStoreCategories', () => {
  it('uses the store-scoped key when storeId is given', () => {
    renderHook(() => useStoreCategories('store-1'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/shoppinglist/stores/store-1/categories',
      expect.any(Function)
    )
  })

  it('passes null as key when storeId is empty', () => {
    renderHook(() => useStoreCategories(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls getStoreCategories with storeId', async () => {
    renderHook(() => useStoreCategories('store-1'))
    await callFetcher()
    expect(mockClient.getStoreCategories).toHaveBeenCalledWith({ storeId: 'store-1' })
  })
})

describe('useItemNames', () => {
  it('uses /shoppinglist/item-names as the SWR key', () => {
    renderHook(() => useItemNames())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/item-names', expect.any(Function))
  })

  it('fetcher calls listItemNames', async () => {
    renderHook(() => useItemNames())
    await callFetcher()
    expect(mockClient.listItemNames).toHaveBeenCalledWith({})
  })
})

describe('useItemCategories', () => {
  it('uses /shoppinglist/item-categories as the SWR key', () => {
    renderHook(() => useItemCategories())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/item-categories', expect.any(Function))
  })

  it('fetcher calls listItemCategories', async () => {
    renderHook(() => useItemCategories())
    await callFetcher()
    expect(mockClient.listItemCategories).toHaveBeenCalledWith({})
  })
})
