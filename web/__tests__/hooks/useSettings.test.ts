import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getSettings: jest.fn(),
    saveSettings: jest.fn()
  }))
}))
jest.mock('@/lib/gen/settings/v1/settings_pb', () => ({
  ...jest.requireActual('@/lib/gen/settings/v1/settings_pb'),
  SettingsService: {}
}))

import useSWR from 'swr'
import { create } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import { useSettings, useSaveSettings } from '@/hooks/useSettings'
import { IntegrationsSchema } from '@/lib/gen/settings/v1/settings_pb'

const mockUseSWR = jest.mocked(useSWR)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({
    data: undefined,
    isLoading: false,
    error: undefined
  })
  mockUseSWR.mockClear()
})

describe('useSettings', () => {
  it('uses /settings as key', () => {
    renderHook(() => useSettings())
    expect(mockUseSWR).toHaveBeenCalledWith('/settings', expect.any(Function), {
      revalidateOnFocus: false,
      revalidateOnReconnect: false
    })
  })

  it('returns SWR result', () => {
    const mockData = {
      integrations: { steamUserId: 'id' }
    }
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValueOnce({
      data: mockData,
      isLoading: false,
      error: undefined
    })
    const { result } = renderHook(() => useSettings())
    expect(result.current.data).toEqual(mockData)
  })
})

describe('useSaveSettings', () => {
  it('returns a function that calls client.saveSettings', () => {
    const mockSaveSettings = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      saveSettings: mockSaveSettings
    })

    const integrations = create(IntegrationsSchema, {
      steamUserId: 'id'
    })

    const { result } = renderHook(() => useSaveSettings())
    result.current(integrations)
    expect(mockSaveSettings).toHaveBeenCalledWith({ integrations })
  })
})
