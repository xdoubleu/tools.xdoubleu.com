import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn(), mutate: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    listOAuthConnections: jest.fn(),
    disconnectOAuthConnection: jest.fn(),
    getProviderOptions: jest.fn(),
    setProviderConfig: jest.fn(),
    getNotificationSettings: jest.fn(),
    updateNotificationSettings: jest.fn()
  }))
}))
jest.mock('@/lib/gen/observability/v1/observability_pb', () => ({
  ObservabilityService: {},
  ProviderConfigSchema: {}
}))

import useSWR, { mutate } from 'swr'
import { createServiceClient } from '@/lib/client'
import type { ProviderConfigInput } from '@/hooks/useMonitoring'
import {
  useOAuthConnections,
  useDisconnectOAuthConnection,
  useProviderOptions,
  useSetProviderConfig,
  useNotificationSettings,
  useUpdateNotificationSettings
} from '@/hooks/useMonitoring'

const mockUseSWR = jest.mocked(useSWR)
const mockMutate = jest.mocked(mutate)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  jest.clearAllMocks()
  mockUseSWR.mockReturnValue(
    // @ts-expect-error -- partial SWRResponse for test purposes
    { data: undefined, isLoading: false, error: undefined }
  )
})

describe('useOAuthConnections', () => {
  it('uses the oauth-connections SWR key', () => {
    renderHook(() => useOAuthConnections())
    expect(mockUseSWR).toHaveBeenCalledWith('/monitoring/oauth-connections', expect.any(Function))
  })
})

describe('useNotificationSettings', () => {
  it('uses the notification-settings SWR key', () => {
    renderHook(() => useNotificationSettings())
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/monitoring/notification-settings',
      expect.any(Function)
    )
  })
})

describe('useDisconnectOAuthConnection', () => {
  it('calls client.disconnectOAuthConnection then revalidates connections', async () => {
    const disconnectOAuthConnection = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ disconnectOAuthConnection })

    const { result } = renderHook(() => useDisconnectOAuthConnection())
    await result.current('github')

    expect(disconnectOAuthConnection).toHaveBeenCalledWith({ provider: 'github' })
    expect(mockMutate).toHaveBeenCalledWith('/monitoring/oauth-connections')
  })
})

describe('useProviderOptions', () => {
  it('calls client.getProviderOptions with a defaulted sentryOrg', () => {
    const getProviderOptions = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ getProviderOptions })

    const { result } = renderHook(() => useProviderOptions())
    result.current('sentry')

    expect(getProviderOptions).toHaveBeenCalledWith({ provider: 'sentry', sentryOrg: '' })
  })

  it('passes an explicit sentryOrg through', () => {
    const getProviderOptions = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ getProviderOptions })

    const { result } = renderHook(() => useProviderOptions())
    result.current('sentry', 'acme')

    expect(getProviderOptions).toHaveBeenCalledWith({ provider: 'sentry', sentryOrg: 'acme' })
  })
})

describe('useSetProviderConfig', () => {
  it('calls client.setProviderConfig then revalidates connections', async () => {
    const setProviderConfig = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ setProviderConfig })

    const { result } = renderHook(() => useSetProviderConfig())
    const config: ProviderConfigInput = { config: { case: 'github', value: { repo: 'x/y' } } }
    await result.current('github', config)

    expect(setProviderConfig).toHaveBeenCalledWith({ provider: 'github', config })
    expect(mockMutate).toHaveBeenCalledWith('/monitoring/oauth-connections')
  })
})

describe('useUpdateNotificationSettings', () => {
  it('calls client.updateNotificationSettings then revalidates settings', async () => {
    const updateNotificationSettings = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ updateNotificationSettings })

    const { result } = renderHook(() => useUpdateNotificationSettings())
    await result.current('sentry_issues', true)

    expect(updateNotificationSettings).toHaveBeenCalledWith({
      sourceKey: 'sentry_issues',
      enabled: true
    })
    expect(mockMutate).toHaveBeenCalledWith('/monitoring/notification-settings')
  })
})
