import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
let mockStatus = 'sent'
jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: jest.fn(async () => ({ status: () => mockStatus })),
  flushOutbox: jest.fn().mockResolvedValue(undefined),
  sendWrite: jest.fn(async (pending: Promise<{ status: () => string }>) => {
    const status = (await pending).status()
    if (status === 'failed') throw new Error('The server rejected the change')
    return status
  })
}))

import useSWR from 'swr'
import { enqueueWrite, flushOutbox } from '@/lib/offline/outbox'
import {
  createMealWrite,
  deleteMealWrite,
  moveMealWrite,
  updateMealWrite,
  updatePlanWrite
} from '@/lib/mealplans/offlineWrites'
import {
  useMealPlans,
  useMealPlan,
  useMealSuggestions,
  useUpdatePlan,
  useRotateICalToken,
  useAddMeal,
  useUpdateMeal,
  useDeleteMeal,
  useMoveMeal
} from '@/hooks/useMealPlans'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
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

describe('useMealSuggestions', () => {
  it('builds a key from plan, date and slot', () => {
    renderHook(() => useMealSuggestions('plan-1', '2024-01-22', 'noon'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/mealplans/plan-1/suggest?d=2024-01-22&s=noon',
      expect.any(Function)
    )
  })

  it('passes null as key when any argument is missing', () => {
    renderHook(() => useMealSuggestions('plan-1', '', 'noon'))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useRotateICalToken', () => {
  it('calls client.rotateICalToken with the plan id', () => {
    const rotateICalToken = jest.fn()
    jest.mocked(jest.requireMock('@/lib/client').createServiceClient).mockReturnValueOnce({
      rotateICalToken
    })
    const { result } = renderHook(() => useRotateICalToken())
    result.current('p1')
    expect(rotateICalToken).toHaveBeenCalledWith({ id: 'p1' })
  })
})

describe('mutation hooks queue writes and send them', () => {
  beforeEach(() => {
    mockStatus = 'sent'
    jest.mocked(enqueueWrite).mockClear()
    jest.mocked(flushOutbox).mockClear()
  })

  it('useUpdatePlan queues the edit and throws when the server rejects it', async () => {
    const update = renderHook(() => useUpdatePlan()).result.current
    await update({ id: 'p1', name: 'Week' })
    expect(enqueueWrite).toHaveBeenCalledWith(updatePlanWrite, { id: 'p1', name: 'Week' })

    mockStatus = 'failed'
    await expect(update({ id: 'p1' })).rejects.toThrow('The server rejected the change')
  })

  it('useAddMeal queues a meal with a client ID and the recipe name', async () => {
    const add = renderHook(() => useAddMeal()).result.current
    await add({ planId: 'p1', mealDate: '2024-01-22', mealSlot: 'noon' }, 'Soup')

    expect(enqueueWrite).toHaveBeenCalledWith(
      createMealWrite,
      expect.objectContaining({ planId: 'p1', id: expect.stringMatching(/^[0-9a-f-]{36}$/) }),
      'Soup'
    )
    expect(flushOutbox).toHaveBeenCalled()
  })

  it('useUpdateMeal, useDeleteMeal and useMoveMeal queue their writes without throwing', async () => {
    mockStatus = 'failed'
    const update = renderHook(() => useUpdateMeal()).result.current
    const remove = renderHook(() => useDeleteMeal()).result.current
    const move = renderHook(() => useMoveMeal()).result.current

    await update({ planId: 'p1', mealId: 'm1' }, 'Stew')
    await remove({ planId: 'p1', mealId: 'm1' })
    await move({ planId: 'p1', mealId: 'm1', newDate: '2024-01-23', newSlot: 'noon' })

    expect(enqueueWrite).toHaveBeenCalledWith(
      updateMealWrite,
      { planId: 'p1', mealId: 'm1' },
      'Stew'
    )
    expect(enqueueWrite).toHaveBeenCalledWith(deleteMealWrite, { planId: 'p1', mealId: 'm1' })
    expect(enqueueWrite).toHaveBeenCalledWith(moveMealWrite, {
      planId: 'p1',
      mealId: 'm1',
      newDate: '2024-01-23',
      newSlot: 'noon'
    })
    expect(flushOutbox).toHaveBeenCalledTimes(3)
  })
})
