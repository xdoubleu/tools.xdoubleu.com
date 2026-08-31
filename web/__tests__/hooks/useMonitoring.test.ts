import { renderHook, act } from '@testing-library/react'
import { unstable_serialize } from 'swr'

const mockMutate = jest.fn()
const mockDisconnectOAuthConnection = jest.fn()
const mockGetProviderOptions = jest.fn()
const mockSetProviderConfig = jest.fn()
const mockTriggerStorageScan = jest.fn()
const mockResolveSentryIssue = jest.fn()
const mockDismissSecurityAlert = jest.fn()
const mockGetNotificationSettings = jest.fn()
const mockUpdateNotificationSettings = jest.fn()

jest.mock('swr', () => ({
  __esModule: true,
  default: jest.fn(),
  mutate: (...args: unknown[]) => mockMutate(...args),
  unstable_serialize: jest.requireActual('swr').unstable_serialize
}))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getJobStats: jest.fn(),
    getStorageStats: jest.fn(),
    triggerStorageScan: (...args: unknown[]) => mockTriggerStorageScan(...args),
    getDatabaseStats: jest.fn(),
    getDatabaseSizeHistory: jest.fn(),
    getFailingPullRequests: jest.fn(),
    getSecurityAlerts: jest.fn(),
    dismissSecurityAlert: (...args: unknown[]) => mockDismissSecurityAlert(...args),
    getSentryIssues: jest.fn(),
    getSlowTransactions: jest.fn(),
    getTransactionLatencyHistory: jest.fn(),
    resolveSentryIssue: (...args: unknown[]) => mockResolveSentryIssue(...args),
    getHostMetrics: jest.fn(),
    getAlertStates: jest.fn(),
    getLogs: jest.fn(),
    listOAuthConnections: jest.fn(),
    disconnectOAuthConnection: (...args: unknown[]) => mockDisconnectOAuthConnection(...args),
    getProviderOptions: (...args: unknown[]) => mockGetProviderOptions(...args),
    setProviderConfig: (...args: unknown[]) => mockSetProviderConfig(...args),
    getNotificationSettings: (...args: unknown[]) => mockGetNotificationSettings(...args),
    updateNotificationSettings: (...args: unknown[]) => mockUpdateNotificationSettings(...args)
  }))
}))
jest.mock('@/lib/gen/observability/v1/observability_pb', () => ({
  ObservabilityService: {},
  ProviderConfigSchema: {}
}))

import useSWR from 'swr'
import {
  useJobStats,
  useStorageStats,
  useTriggerStorageScan,
  useDatabaseStats,
  useDatabaseSizeHistory,
  useFailingPullRequests,
  useSentryIssues,
  useSlowTransactions,
  useTransactionLatencyHistory,
  useResolveSentryIssue,
  useDismissSecurityAlert,
  useHostMetrics,
  useAlertStates,
  useLogs,
  useOAuthConnections,
  useDisconnectOAuthConnection,
  useProviderOptions,
  useSetProviderConfig,
  useNotificationSettings,
  useUpdateNotificationSettings
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

  it('keys storage stats statically', () => {
    renderHook(() => useStorageStats())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringStorageStats, expect.any(Function))
  })

  it('keys database stats by window', () => {
    renderHook(() => useDatabaseStats(30))
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringDatabaseStats(30),
      expect.any(Function)
    )
  })

  it('keys database size history by window', () => {
    renderHook(() => useDatabaseSizeHistory(30))
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringDatabaseSizeHistory(30),
      expect.any(Function)
    )
  })

  it('keys failing pull requests statically', () => {
    renderHook(() => useFailingPullRequests())
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringFailingPullRequests,
      expect.any(Function)
    )
  })

  it('keys sentry issues statically', () => {
    renderHook(() => useSentryIssues())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringSentryIssues, expect.any(Function))
  })

  it('keys slow transactions statically', () => {
    renderHook(() => useSlowTransactions())
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringSlowTransactions,
      expect.any(Function)
    )
  })

  it('keys transaction latency history by window', () => {
    renderHook(() => useTransactionLatencyHistory(30))
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringTransactionLatencyHistory(30),
      expect.any(Function)
    )
  })

  it('keys host metrics statically', () => {
    renderHook(() => useHostMetrics())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringHostMetrics, expect.any(Function))
  })

  it('keys alert states statically', () => {
    renderHook(() => useAlertStates())
    expect(mockUseSWR).toHaveBeenCalledWith(swrKeys.monitoringAlertStates, expect.any(Function))
  })

  it('keys logs by source and min level', () => {
    renderHook(() => useLogs('api', 'warn'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringLogs('api', 'warn'),
      expect.any(Function)
    )
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

  it('keys notification settings statically', () => {
    renderHook(() => useNotificationSettings())
    expect(mockUseSWR).toHaveBeenCalledWith(
      swrKeys.monitoringNotificationSettings,
      expect.any(Function)
    )
  })
})

describe('useUpdateNotificationSettings', () => {
  it('updates a source and revalidates notification settings', async () => {
    mockUpdateNotificationSettings.mockResolvedValue({})
    const { result } = renderHook(() => useUpdateNotificationSettings())

    await act(async () => {
      await result.current('sentry_issues', false)
    })

    expect(mockUpdateNotificationSettings).toHaveBeenCalledWith({
      sourceKey: 'sentry_issues',
      enabled: false
    })
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringNotificationSettings)
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

describe('useResolveSentryIssue', () => {
  it('resolves the given issue and revalidates the sentry issues list', async () => {
    mockResolveSentryIssue.mockResolvedValue({})
    const { result } = renderHook(() => useResolveSentryIssue())

    await act(async () => {
      await result.current('42')
    })

    expect(mockResolveSentryIssue).toHaveBeenCalledWith({ issueId: '42' })
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringSentryIssues)
  })
})

describe('useDismissSecurityAlert', () => {
  it('dismisses the given alert and revalidates the security alerts list', async () => {
    mockDismissSecurityAlert.mockResolvedValue({})
    const { result } = renderHook(() => useDismissSecurityAlert())

    await act(async () => {
      await result.current(1, 83n, 'no_bandwidth')
    })

    expect(mockDismissSecurityAlert).toHaveBeenCalledWith({
      alertType: 1,
      alertNumber: 83n,
      reason: 'no_bandwidth'
    })
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringSecurityAlerts)
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
    expect(mockMutate).toHaveBeenCalledWith(swrKeys.monitoringFailingPullRequests)
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
