import { renderHook } from '@testing-library/react'

vi.mock('swr', () => ({ __esModule: true, default: vi.fn(), mutate: vi.fn() }))
vi.mock('@/lib/client', () => ({ createServiceClient: vi.fn() }))
vi.mock('@/lib/gen/observability/v1/observability_pb', () => ({ ObservabilityService: {} }))

import useSWR, { mutate } from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useNotificationSettings,
  useUpdateNotificationSettings
} from '@/hooks/useNotificationSettings'

const mockUseSWR = vi.mocked(useSWR)
const mockCreateServiceClient = vi.mocked(createServiceClient)

beforeEach(() => {
  vi.clearAllMocks()
  mockUseSWR.mockReturnValue(
    // @ts-expect-error -- partial SWRResponse for test purposes
    { data: undefined, isLoading: false, error: undefined }
  )
})

describe('useNotificationSettings', () => {
  it('fetches the settings under the notification-settings key', () => {
    const getNotificationSettings = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ getNotificationSettings })

    renderHook(() => useNotificationSettings())
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/monitoring/notification-settings',
      expect.any(Function)
    )

    const fetcher = mockUseSWR.mock.calls[0]![1]!
    fetcher()
    expect(getNotificationSettings).toHaveBeenCalledWith({})
  })
})

describe('useUpdateNotificationSettings', () => {
  it('calls client.updateNotificationSettings then revalidates settings', async () => {
    const updateNotificationSettings = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- partial client shape
    mockCreateServiceClient.mockReturnValue({ updateNotificationSettings })

    const { result } = renderHook(() => useUpdateNotificationSettings())
    await result.current('sentry_issues', true)

    expect(updateNotificationSettings).toHaveBeenCalledWith({
      sourceKey: 'sentry_issues',
      enabled: true
    })
    expect(vi.mocked(mutate)).toHaveBeenCalledWith('/monitoring/notification-settings')
  })
})
