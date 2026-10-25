import { renderHook } from '@testing-library/react'

vi.mock('swr', () => ({ __esModule: true, default: vi.fn() }))
const mockClient = {
  listRecipes: vi.fn().mockResolvedValue({ recipes: [{ id: 'r-1' }], hasMore: true })
}
vi.mock('@/lib/client', () => ({
  transport: {},
  createServiceClient: vi.fn(() => mockClient)
}))
let mockStatus = 'sent'
vi.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: vi.fn(async () => ({ status: () => mockStatus })),
  // Mirrors the real sendWrite over the mocked handle.
  sendWrite: vi.fn(async (pending: Promise<{ status: () => string }>) => {
    const status = (await pending).status()
    if (status === 'failed') throw new Error('The server rejected the change')
    return status
  })
}))

import useSWR from 'swr'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import {
  createRecipeWrite,
  deleteRecipeWrite,
  updateRecipeWrite
} from '@/lib/recipes/offlineWrites'
import {
  useRecipes,
  useAllRecipes,
  useRecipe,
  useCreateRecipe,
  useUpdateRecipe,
  useDeleteRecipe,
  useFetchRecipesPage
} from '@/hooks/useRecipes'

const mockUseSWR = vi.mocked(useSWR)

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

function isFetcher(fn: unknown): fn is () => Promise<unknown> {
  return typeof fn === 'function'
}

function swrFetcher() {
  const fn: unknown = mockUseSWR.mock.calls[0][1]
  if (!isFetcher(fn)) throw new Error('useSWR was called without a fetcher')
  return fn
}

describe('useAllRecipes', () => {
  it('pages through every recipe until hasMore is false', async () => {
    mockClient.listRecipes
      .mockResolvedValueOnce({ recipes: [{ id: 'r-1' }, { id: 'r-2' }], hasMore: true })
      .mockResolvedValueOnce({ recipes: [{ id: 'r-3' }], hasMore: false })
    renderHook(() => useAllRecipes())
    expect(mockUseSWR).toHaveBeenCalledWith('/recipes?all', expect.any(Function))
    await expect(swrFetcher()()).resolves.toEqual([{ id: 'r-1' }, { id: 'r-2' }, { id: 'r-3' }])
    expect(mockClient.listRecipes).toHaveBeenNthCalledWith(1, { limit: 200, offset: 0 })
    expect(mockClient.listRecipes).toHaveBeenNthCalledWith(2, { limit: 200, offset: 2 })
  })

  it('stops on an empty page even if hasMore is set', async () => {
    mockClient.listRecipes.mockResolvedValueOnce({ recipes: [], hasMore: true })
    renderHook(() => useAllRecipes())
    await expect(swrFetcher()()).resolves.toEqual([])
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

describe('mutation hooks queue writes and send them', () => {
  beforeEach(() => {
    mockStatus = 'sent'
  })

  it('useCreateRecipe queues with a client ID and resolves to it', async () => {
    const { result } = renderHook(() => useCreateRecipe())
    const { id, synced } = await result.current({ name: 'Soup' })

    expect(id).toMatch(/^[0-9a-f-]{36}$/)
    expect(synced).toBe(true)
    expect(enqueueWrite).toHaveBeenCalledWith(
      createRecipeWrite,
      expect.objectContaining({ id, name: 'Soup' })
    )
    expect(sendWrite).toHaveBeenCalled()
  })

  it('useCreateRecipe reports a create still queued offline as unsynced', async () => {
    mockStatus = 'queued'
    const { result } = renderHook(() => useCreateRecipe())
    expect((await result.current({ name: 'Soup' })).synced).toBe(false)
  })

  it('throws when the server rejects a create or update', async () => {
    mockStatus = 'failed'
    const create = renderHook(() => useCreateRecipe()).result.current
    const update = renderHook(() => useUpdateRecipe()).result.current
    await expect(create({ name: 'Soup' })).rejects.toThrow('The server rejected the change')
    await expect(update({ id: 'r-1' })).rejects.toThrow('The server rejected the change')
  })

  it('useUpdateRecipe queues the update', async () => {
    const { result } = renderHook(() => useUpdateRecipe())
    await result.current({ id: 'r-1', name: 'Stew' })
    expect(enqueueWrite).toHaveBeenCalledWith(updateRecipeWrite, { id: 'r-1', name: 'Stew' })
  })

  it('useDeleteRecipe queues the delete and sends it', async () => {
    const { result } = renderHook(() => useDeleteRecipe())
    await result.current({ id: 'r-1' })
    expect(enqueueWrite).toHaveBeenCalledWith(deleteRecipeWrite, { id: 'r-1' })
    expect(sendWrite).toHaveBeenCalled()
  })

  it('useDeleteRecipe resolves for a delete still queued offline', async () => {
    mockStatus = 'queued'
    const { result } = renderHook(() => useDeleteRecipe())
    await expect(result.current({ id: 'r-1' })).resolves.toBeUndefined()
  })

  it('useDeleteRecipe throws when the server rejects the delete', async () => {
    mockStatus = 'failed'
    const { result } = renderHook(() => useDeleteRecipe())
    await expect(result.current({ id: 'r-1' })).rejects.toThrow('The server rejected the change')
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
