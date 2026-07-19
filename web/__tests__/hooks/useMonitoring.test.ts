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
jest.mock('@/lib/gen/observability/v1/observability_pb', () => ({
  ObservabilityService: {}
}))

import useSWR from 'swr'
import {
  useJobStats,
  useUsageStats,
  useStorageStats,
  useDatabaseStats
} from '@/hooks/useMonitoring'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  mockUseSWR.mockReturnValue(
    // @ts-expect-error -- mock returns a partial SWRResponse for test purposes
    { data: undefined, isLoading: false, error: undefined }
  )
  mockUseSWR.mockClear()
})

describe('useMonitoring', () => {
  it('keys job stats by window', () => {
    renderHook(() => useJobStats(7))
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringJobStats(7), expect.any(Function))
  })

  it('keys usage stats by window', () => {
    renderHook(() => useUsageStats(30))
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringUsageStats(30), expect.any(Function))
  })

  it('keys storage stats statically', () => {
    renderHook(() => useStorageStats())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringStorageStats, expect.any(Function))
  })

  it('keys database stats statically', () => {
    renderHook(() => useDatabaseStats())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringDatabaseStats, expect.any(Function))
  })

  it('distinct window keys do not collide', () => {
    expect(unstable_serialize(swrKeys.monitoringJobStats(7))).not.toBe(
      unstable_serialize(swrKeys.monitoringJobStats(30))
    )
  })
})
