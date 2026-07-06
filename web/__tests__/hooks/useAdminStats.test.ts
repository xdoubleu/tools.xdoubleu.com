import { renderHook } from '@testing-library/react'
import { unstable_serialize } from 'swr'

jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  unstable_serialize: jest.requireActual('swr').unstable_serialize
}))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getJobStats: jest.fn(),
    getUsageStats: jest.fn(),
    getStorageStats: jest.fn(),
    getDatabaseStats: jest.fn()
  }))
}))
jest.mock('@/lib/gen/admin/v1/admin_pb', () => ({
  AdminService: {}
}))

import useSWR from 'swr'
import {
  useJobStats,
  useUsageStats,
  useStorageStats,
  useDatabaseStats
} from '@/hooks/useAdminStats'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  mockUseSWR.mockReturnValue(
    // @ts-expect-error -- mock returns a partial SWRResponse for test purposes
    { data: undefined, isLoading: false, error: undefined }
  )
  mockUseSWR.mockClear()
})

describe('useAdminStats', () => {
  it('keys job stats by window', () => {
    renderHook(() => useJobStats(7))
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.adminJobStats(7), expect.any(Function))
  })

  it('keys usage stats by window', () => {
    renderHook(() => useUsageStats(30))
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.adminUsageStats(30), expect.any(Function))
  })

  it('keys storage stats statically', () => {
    renderHook(() => useStorageStats())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.adminStorageStats, expect.any(Function))
  })

  it('keys database stats statically', () => {
    renderHook(() => useDatabaseStats())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.adminDatabaseStats, expect.any(Function))
  })

  it('distinct window keys do not collide', () => {
    expect(unstable_serialize(swrKeys.adminJobStats(7))).not.toBe(
      unstable_serialize(swrKeys.adminJobStats(30))
    )
  })
})
