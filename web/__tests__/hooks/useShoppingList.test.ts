import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockClient = {
  getCustomList: jest.fn().mockResolvedValue({}),
  getMealPlanExportItems: jest.fn().mockResolvedValue({ items: [] }),
  getPlanIngredientGroups: jest.fn().mockResolvedValue({ groups: [] }),
  listCategories: jest.fn().mockResolvedValue({}),
  listStores: jest.fn().mockResolvedValue({}),
  getStoreCategories: jest.fn().mockResolvedValue({}),
  listItemNames: jest.fn().mockResolvedValue({}),
  listItemCategories: jest.fn().mockResolvedValue({}),
  listPlans: jest.fn().mockResolvedValue({ plans: [] }),
  listAccessibleLists: jest.fn().mockResolvedValue({ owners: [] }),
  listShoppingListShares: jest.fn().mockResolvedValue({ shares: [] }),
  shareShoppingList: jest.fn().mockResolvedValue({}),
  unshareShoppingList: jest.fn().mockResolvedValue({})
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/shoppinglist/v1/shoppinglist_pb', () => ({
  ShoppingListService: {}
}))
jest.mock('@/lib/gen/mealplans/v1/mealplans_pb', () => ({
  MealPlansService: {}
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
  useItemCategories,
  useAllMealPlanExportItems,
  useAllPlanIngredientGroups,
  useAccessibleLists,
  useShoppingListShares,
  useShareShoppingList,
  useUnshareShoppingList
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
  it('uses /shoppinglist as the SWR key, scoped by owner', () => {
    renderHook(() => useCustomList())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist?owner=', expect.any(Function))
  })

  it('includes the owner in the key when given', () => {
    renderHook(() => useCustomList('owner-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist?owner=owner-1', expect.any(Function))
  })

  it('fetcher calls getCustomList with the owner', async () => {
    renderHook(() => useCustomList('owner-1'))
    await callFetcher()
    expect(mockClient.getCustomList).toHaveBeenCalledWith({ ownerUserId: 'owner-1' })
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
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/groups/plan-3', expect.any(Function))
  })

  it('passes null as key when planId is empty', () => {
    renderHook(() => usePlanIngredientGroups(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useCategories', () => {
  it('uses /shoppinglist/categories as the SWR key, scoped by owner', () => {
    renderHook(() => useCategories())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/categories?owner=', expect.any(Function))
  })

  it('fetcher calls listCategories with the owner', async () => {
    renderHook(() => useCategories('owner-1'))
    await callFetcher()
    expect(mockClient.listCategories).toHaveBeenCalledWith({ ownerUserId: 'owner-1' })
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

describe('useAllMealPlanExportItems', () => {
  it('uses /shoppinglist/export/all as key with sorted excluded groups', () => {
    renderHook(() => useAllMealPlanExportItems(['sauce', 'pasta']))
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/shoppinglist/export/all?excluded=pasta,sauce',
      expect.any(Function)
    )
  })

  it('uses empty excluded string when no groups excluded', () => {
    renderHook(() => useAllMealPlanExportItems())
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/shoppinglist/export/all?excluded=',
      expect.any(Function)
    )
  })

  it('fetcher returns empty items when no plans exist', async () => {
    mockClient.listPlans.mockResolvedValueOnce({ plans: [] })
    renderHook(() => useAllMealPlanExportItems())
    const fetcher = mockUseSWR.mock.calls[0]?.[1]
    const result = typeof fetcher === 'function' ? await fetcher() : undefined
    expect(result).toEqual({ items: [] })
    expect(mockClient.getMealPlanExportItems).not.toHaveBeenCalled()
  })

  it('fetcher calls getMealPlanExportItems for each plan and merges results', async () => {
    mockClient.listPlans.mockResolvedValueOnce({
      plans: [{ id: 'plan-1' }, { id: 'plan-2' }]
    })
    mockClient.getMealPlanExportItems
      .mockResolvedValueOnce({ items: [{ name: 'garlic', amount: '2', unit: 'cloves' }] })
      .mockResolvedValueOnce({ items: [{ name: 'onion', amount: '1', unit: 'pc' }] })
    renderHook(() => useAllMealPlanExportItems())
    const fetcher = mockUseSWR.mock.calls[0]?.[1]
    const result = typeof fetcher === 'function' ? await fetcher() : undefined
    expect(mockClient.getMealPlanExportItems).toHaveBeenCalledTimes(2)
    expect(result).toEqual({
      items: [
        { name: 'garlic', amount: '2', unit: 'cloves' },
        { name: 'onion', amount: '1', unit: 'pc' }
      ]
    })
  })
})

describe('sharing hooks', () => {
  it('useAccessibleLists uses its key and fetches owners', async () => {
    renderHook(() => useAccessibleLists())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/accessible', expect.any(Function))
    await callFetcher()
    expect(mockClient.listAccessibleLists).toHaveBeenCalledWith({})
  })

  it('useShoppingListShares uses its key and fetches shares', async () => {
    renderHook(() => useShoppingListShares())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/shares', expect.any(Function))
    await callFetcher()
    expect(mockClient.listShoppingListShares).toHaveBeenCalledWith({})
  })

  it('useShareShoppingList calls shareShoppingList with contact and permission', () => {
    const { result } = renderHook(() => useShareShoppingList())
    result.current('u-1', true)
    expect(mockClient.shareShoppingList).toHaveBeenCalledWith({
      contactUserId: 'u-1',
      canEdit: true
    })
  })

  it('useUnshareShoppingList calls unshareShoppingList with the target', () => {
    const { result } = renderHook(() => useUnshareShoppingList())
    result.current('u-2')
    expect(mockClient.unshareShoppingList).toHaveBeenCalledWith({ targetUserId: 'u-2' })
  })
})

describe('useAllPlanIngredientGroups', () => {
  it('uses /shoppinglist/groups/all as the SWR key', () => {
    renderHook(() => useAllPlanIngredientGroups())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/groups/all', expect.any(Function))
  })

  it('fetcher returns empty groups when no plans exist', async () => {
    mockClient.listPlans.mockResolvedValueOnce({ plans: [] })
    renderHook(() => useAllPlanIngredientGroups())
    const fetcher = mockUseSWR.mock.calls[0]?.[1]
    const result = typeof fetcher === 'function' ? await fetcher() : undefined
    expect(result).toEqual({ groups: [] })
    expect(mockClient.getPlanIngredientGroups).not.toHaveBeenCalled()
  })

  it('fetcher merges and deduplicates groups across plans by groupName', async () => {
    mockClient.listPlans.mockResolvedValueOnce({
      plans: [{ id: 'plan-1' }, { id: 'plan-2' }]
    })
    mockClient.getPlanIngredientGroups
      .mockResolvedValueOnce({
        groups: [
          { recipeName: 'Pasta', groupName: 'Sauce' },
          { recipeName: 'Pasta', groupName: 'Base' }
        ]
      })
      .mockResolvedValueOnce({
        groups: [
          { recipeName: 'Soup', groupName: 'Sauce' }, // duplicate groupName — deduplicated
          { recipeName: 'Soup', groupName: 'Broth' }
        ]
      })
    renderHook(() => useAllPlanIngredientGroups())
    const fetcher = mockUseSWR.mock.calls[0]?.[1]
    const result = typeof fetcher === 'function' ? await fetcher() : undefined
    expect(result).toEqual({
      groups: [
        { recipeName: 'Pasta', groupName: 'Sauce' },
        { recipeName: 'Pasta', groupName: 'Base' },
        { recipeName: 'Soup', groupName: 'Broth' }
      ]
    })
  })
})
