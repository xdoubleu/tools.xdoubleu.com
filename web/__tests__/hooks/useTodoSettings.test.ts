import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({ getSettings: jest.fn().mockResolvedValue({}) }))
}))
jest.mock('@/lib/gen/todos/v1/settings_pb', () => ({ SettingsService: {} }))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { useTodoSettings } from '@/hooks/useTodoSettings'

const mockUseSWR = useSWR as jest.Mock
const mockCreateServiceClient = createServiceClient as jest.Mock

beforeEach(() => {
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
  mockCreateServiceClient.mockClear()
})

describe('useTodoSettings', () => {
  it('uses /todos/settings as key', () => {
    renderHook(() => useTodoSettings())
    expect(mockUseSWR).toHaveBeenCalledWith('/todos/settings', expect.any(Function))
  })

  it('fetcher calls client.getSettings', async () => {
    const mockGetSettings = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValueOnce({ getSettings: mockGetSettings })
    renderHook(() => useTodoSettings())
    const fetcher = mockUseSWR.mock.calls[0][1]
    await fetcher()
    expect(mockGetSettings).toHaveBeenCalledWith({})
  })

  it('returns SWR result', () => {
    const mockData = { sections: [] }
    mockUseSWR.mockReturnValueOnce({ data: mockData, isLoading: false, error: undefined })
    const { result } = renderHook(() => useTodoSettings())
    expect(result.current.data).toEqual(mockData)
  })
})
