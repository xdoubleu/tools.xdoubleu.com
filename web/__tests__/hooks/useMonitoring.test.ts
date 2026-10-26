import { renderHook } from '@testing-library/react'

vi.mock('swr', () => ({ __esModule: true, default: vi.fn(), mutate: vi.fn() }))
vi.mock('@/lib/client', () => ({
  createServiceClient: vi.fn(() => ({
    listOAuthConnections: vi.fn(),
    disconnectOAuthConnection: vi.fn(),
    getProviderOptions: vi.fn(),
    setProviderConfig: vi.fn(),
    getAutomatedActions: vi.fn()
  }))
}))
vi.mock('@/lib/gen/observability/v1/observability_pb', () => ({
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
  useAutomatedActions
} from '@/hooks/useMonitoring'

const mockUseSWR = vi.mocked(useSWR)
const mockMutate = vi.mocked(mutate)
const mockCreateServiceClient = vi.mocked(createServiceClient)

beforeEach(() => {
  vi.clearAllMocks()
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

describe('useAutomatedActions', () => {
  it('uses the automated-actions SWR key and calls getAutomatedActions with an empty request', () => {
    const getAutomatedActions = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ getAutomatedActions })

    renderHook(() => useAutomatedActions())
    expect(mockUseSWR).toHaveBeenCalledWith('/monitoring/automated-actions', expect.any(Function))

    const fetcher = mockUseSWR.mock.calls[0]![1]!
    fetcher()
    expect(getAutomatedActions).toHaveBeenCalledWith({})
  })
})

describe('useDisconnectOAuthConnection', () => {
  it('calls client.disconnectOAuthConnection then revalidates connections', async () => {
    const disconnectOAuthConnection = vi.fn().mockResolvedValue({})
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
    const getProviderOptions = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ getProviderOptions })

    const { result } = renderHook(() => useProviderOptions())
    result.current('sentry')

    expect(getProviderOptions).toHaveBeenCalledWith({ provider: 'sentry', sentryOrg: '' })
  })

  it('passes an explicit sentryOrg through', () => {
    const getProviderOptions = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ getProviderOptions })

    const { result } = renderHook(() => useProviderOptions())
    result.current('sentry', 'acme')

    expect(getProviderOptions).toHaveBeenCalledWith({ provider: 'sentry', sentryOrg: 'acme' })
  })
})

describe('useSetProviderConfig', () => {
  it('calls client.setProviderConfig then revalidates connections', async () => {
    const setProviderConfig = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ setProviderConfig })

    const { result } = renderHook(() => useSetProviderConfig())
    const config: ProviderConfigInput = { config: { case: 'github', value: { repo: 'x/y' } } }
    await result.current('github', config)

    expect(setProviderConfig).toHaveBeenCalledWith({ provider: 'github', config })
    expect(mockMutate).toHaveBeenCalledWith('/monitoring/oauth-connections')
  })
})
