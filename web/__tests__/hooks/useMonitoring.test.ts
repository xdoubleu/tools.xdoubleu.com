import { renderHook, act } from '@testing-library/react'
import { unstable_serialize } from 'swr'

const mockMutate = jest.fn()
const mockDisconnectOAuthConnection = jest.fn()
const mockGetProviderOptions = jest.fn()
const mockSetProviderConfig = jest.fn()
const mockTriggerStorageScan = jest.fn()

jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  mutate: (...args: unknown[]) => mockMutate(...args),
  unstable_serialize: jest.requireActual('swr').unstable_serialize
}))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getJobStats: jest.fn(),
    getUsageStats: jest.fn(),
    getStorageStats: jest.fn(),
    triggerStorageScan: (...args: unknown[]) => mockTriggerStorageScan(...args),
    getDatabaseStats: jest.fn(),
    getGithubIssues: jest.fn(),
    getSentryIssues: jest.fn(),
    getDeployStatus: jest.fn(),
    listOAuthConnections: jest.fn(),
    disconnectOAuthConnection: (...args: unknown[]) => mockDisconnectOAuthConnection(...args),
    getProviderOptions: (...args: unknown[]) => mockGetProviderOptions(...args),
    setProviderConfig: (...args: unknown[]) => mockSetProviderConfig(...args)
  }))
}))
jest.mock('@/lib/gen/observability/v1/observability_pb', () => ({
  ObservabilityService: {},
  ProviderConfigSchema: {}
}))

import useSWR from 'swr'
import {
  useJobStats,
  useUsageStats,
  useStorageStats,
  useTriggerStorageScan,
  useDatabaseStats,
  useGithubIssues,
  useSentryIssues,
  useDeployStatus,
  useOAuthConnections,
  useDisconnectOAuthConnection,
  useProviderOptions,
  useSetProviderConfig
} from '@/hooks/useMonitoring'
import { swrKeys } from '@/lib/swrKeys'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // Invoke the fetcher each hook hands to useSWR so its client call executes.
  // @ts-expect-error -- mock returns a partial SWRResponse for test purposes
  mockUseSWR.mockImplementation((key, fetcher) => {
    if (typeof fetcher === 'function') fetcher(key)
    return { data: undefined, isLoading: false, error: undefined }
  })
})

afterEach(() => {
  mockUseSWR.mockReset()
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

  it('keys github issues statically', () => {
    renderHook(() => useGithubIssues())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringGithubIssues, expect.any(Function))
  })

  it('keys sentry issues statically', () => {
    renderHook(() => useSentryIssues())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringSentryIssues, expect.any(Function))
  })

  it('keys deploy status statically', () => {
    renderHook(() => useDeployStatus())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringDeployStatus, expect.any(Function))
  })

  it('keys oauth connections statically', () => {
    renderHook(() => useOAuthConnections())
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringOAuthConnections,
      expect.any(Function)
    )
  })

  it('distinct window keys do not collide', () => {
    expect(unstable_serialize(swrKeys.monitoringJobStats(7))).not.toBe(
      unstable_serialize(swrKeys.monitoringJobStats(30))
    )
  })
})

describe('useTriggerStorageScan', () => {
  it('runs a live rescan and revalidates storage stats', async () => {
    mockTriggerStorageScan.mockResolvedValue({})
    const { result } = renderHook(() => useTriggerStorageScan())

    await act(async () => {
      await result.current()
    })

    expect(mockTriggerStorageScan).toHaveBeenCalledWith({})
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringStorageStats)
  })
})

describe('useDisconnectOAuthConnection', () => {
  it('disconnects the given provider and revalidates the list', async () => {
    mockDisconnectOAuthConnection.mockResolvedValue({})
    const { result } = renderHook(() => useDisconnectOAuthConnection())

    await act(async () => {
      await result.current('github')
    })

    expect(mockDisconnectOAuthConnection).toHaveBeenCalledWith({ provider: 'github' })
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringOAuthConnections)
  })
})

describe('useProviderOptions', () => {
  it('fetches options for a provider with no sentry org', async () => {
    mockGetProviderOptions.mockResolvedValue({ repos: ['o/r'] })
    const { result } = renderHook(() => useProviderOptions())

    await act(async () => {
      await result.current('github')
    })

    expect(mockGetProviderOptions).toHaveBeenCalledWith({ provider: 'github', sentryOrg: '' })
  })

  it('passes the sentry org through when given', async () => {
    mockGetProviderOptions.mockResolvedValue({ sentryProjects: ['p1'] })
    const { result } = renderHook(() => useProviderOptions())

    await act(async () => {
      await result.current('sentry', 'my-org')
    })

    expect(mockGetProviderOptions).toHaveBeenCalledWith({
      provider: 'sentry',
      sentryOrg: 'my-org'
    })
  })
})

describe('useSetProviderConfig', () => {
  it('saves the config and revalidates the connections list plus the provider data key', async () => {
    mockSetProviderConfig.mockResolvedValue({})
    const { result } = renderHook(() => useSetProviderConfig())

    const config = { config: { case: 'github' as const, value: { repo: 'o/r' } } }
    await act(async () => {
      await result.current('github', config)
    })

    expect(mockSetProviderConfig).toHaveBeenCalledWith({ provider: 'github', config })
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringOAuthConnections)
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringGithubIssues)
  })

  it('does not mutate a data key for an unrecognized provider', async () => {
    mockSetProviderConfig.mockResolvedValue({})
    mockMutate.mockClear()
    const { result } = renderHook(() => useSetProviderConfig())

    await act(async () => {
      await result.current('unknown', {})
    })

    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringOAuthConnections)
    expect(mockMutate).toHaveBeenCalledTimes(1)
  })
})
