import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getSettings: jest.fn(),
    saveSettings: jest.fn()
  }))
}))
jest.mock('@/lib/gen/settings/v1/settings_pb', () => ({
  SettingsService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { useSettings, useSaveSettings } from '@/hooks/useSettings'
import type { Integrations } from '@/lib/gen/settings/v1/settings_pb'

const mockUseSWR = useSWR as jest.Mock
const mockCreateServiceClient = createServiceClient as jest.Mock

beforeEach(() => {
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
      integrations: { steamApiKey: 'key', steamUserId: 'id', hardcoverApiKey: '' }
    }
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
      saveSettings: mockSaveSettings
    })

    const integrations: Integrations = {
      steamApiKey: 'key',
      steamUserId: 'id',
      hardcoverApiKey: 'hkey',
      $typeName: 'settings.v1.Integrations'
    }

    const { result } = renderHook(() => useSaveSettings())
    result.current(integrations)
    expect(mockSaveSettings).toHaveBeenCalledWith({ integrations })
  })
})
