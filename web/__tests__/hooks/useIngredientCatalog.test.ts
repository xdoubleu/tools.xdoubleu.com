import { renderHook } from '@testing-library/react'

vi.mock('swr', () => ({ __esModule: true, default: vi.fn() }))
const mockClient = {
  listCategories: vi.fn().mockResolvedValue({}),
  listItemNames: vi.fn().mockResolvedValue({}),
  listItemCategories: vi.fn().mockResolvedValue({}),
  listIngredientUnits: vi.fn().mockResolvedValue({})
}

vi.mock('@/lib/client', () => ({
  transport: {},
  createServiceClient: vi.fn(() => mockClient)
}))
vi.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: vi.fn().mockResolvedValue(undefined)
}))

import useSWR from 'swr'
import { create } from '@bufbuild/protobuf'
import { CategorySchema } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { enqueueWrite } from '@/lib/offline/outbox'
import { createCategoryWrite, setItemCategoryWrite } from '@/lib/shoppinglist/offlineWrites'
import {
  useCategories,
  useItemNames,
  useItemCategories,
  useItemAliases,
  useIngredientUnits,
  queueCategory,
  queueItemCategory
} from '@/hooks/useIngredientCatalog'

const mockUseSWR = vi.mocked(useSWR)

// Invoke the captured SWR fetcher (typeof guard avoids an unsafe assertion).
async function callFetcher() {
  const fetcher = mockUseSWR.mock.calls[0]?.[1]
  if (typeof fetcher === 'function') await fetcher()
}

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useCategories', () => {
  it('uses /shoppinglist/categories as the SWR key', () => {
    renderHook(() => useCategories())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/categories?owner=', expect.any(Function))
  })

  it('fetcher calls listCategories', async () => {
    renderHook(() => useCategories())
    await callFetcher()
    expect(mockClient.listCategories).toHaveBeenCalledWith({})
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

describe('useItemAliases', () => {
  it('maps merged names to their canonical', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValue({
      data: {
        names: [
          { name: 'paprika', mergedInto: '' },
          { name: 'rode paprika', mergedInto: 'paprika' }
        ]
      }
    })
    const { result } = renderHook(() => useItemAliases())
    expect(result.current).toEqual({ 'rode paprika': 'paprika' })
  })

  it('is empty before the catalog loads', () => {
    const { result } = renderHook(() => useItemAliases())
    expect(result.current).toEqual({})
  })
})

describe('useIngredientUnits', () => {
  it('uses /shoppinglist/ingredient-units as the SWR key', () => {
    renderHook(() => useIngredientUnits())
    expect(mockUseSWR).toHaveBeenCalledWith('/shoppinglist/ingredient-units', expect.any(Function))
  })

  it('fetcher calls listIngredientUnits', async () => {
    renderHook(() => useIngredientUnits())
    await callFetcher()
    expect(mockClient.listIngredientUnits).toHaveBeenCalledWith({})
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

describe('queueCategory', () => {
  beforeEach(() => vi.mocked(enqueueWrite).mockClear())
  const categories = [create(CategorySchema, { id: 'cat-1', name: 'Produce' })]

  it('reuses a same-named category', async () => {
    expect(await queueCategory('produce', categories)).toBe('cat-1')
    expect(enqueueWrite).not.toHaveBeenCalled()
  })

  it('queues a new category with a client ID', async () => {
    const id = await queueCategory('Bakery', categories)
    expect(enqueueWrite).toHaveBeenCalledWith(createCategoryWrite, { id, name: 'Bakery' })
  })
})

describe('queueItemCategory', () => {
  it('queues the catalog write', async () => {
    await queueItemCategory('milk', 'cat-1')
    expect(enqueueWrite).toHaveBeenCalledWith(setItemCategoryWrite, {
      name: 'milk',
      categoryId: 'cat-1'
    })
  })
})
