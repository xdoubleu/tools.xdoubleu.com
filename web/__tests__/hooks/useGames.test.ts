import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    getSteamGame: jest.fn().mockResolvedValue({}),
    getSteamDistribution: jest.fn().mockResolvedValue({}),
    getSteam: jest.fn().mockResolvedValue({}),
    getRecentlyActiveGames: jest.fn().mockResolvedValue({}),
    getIntegrations: jest.fn().mockResolvedValue({}),
    saveIntegrations: jest.fn().mockResolvedValue({})
  }))
}))
jest.mock('@/lib/gen/games/v1/games_pb', () => ({ GamesService: {} }))
jest.mock('@/lib/env', () => ({ getApiUrl: () => 'https://api.test' }))

import useSWR from 'swr'
import {
  useSteam,
  useSteamGame,
  useSteamDistribution,
  useSteamProgress,
  useRecentlyActiveGames,
  useRefreshSteam,
  useIntegrations,
  useSaveIntegrations
} from '@/hooks/useGames'
import { createServiceClient } from '@/lib/client'

const mockUseSWR = jest.mocked(useSWR)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
  mockCreateServiceClient.mockClear()
})

describe('useSteam', () => {
  it('uses /games as key', () => {
    renderHook(() => useSteam())
    expect(mockUseSWR).toHaveBeenCalledWith('/games', expect.any(Function))
  })
})

describe('useRecentlyActiveGames', () => {
  it('uses /games/recent as key', () => {
    renderHook(() => useRecentlyActiveGames())
    expect(mockUseSWR).toHaveBeenCalledWith('/games/recent', expect.any(Function))
  })

  it('fetcher calls client.getRecentlyActiveGames', async () => {
    const mockClient = { getRecentlyActiveGames: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useRecentlyActiveGames())
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getRecentlyActiveGames).toHaveBeenCalledWith({})
  })
})

describe('useSteamGame', () => {
  it('uses correct key when gameId is provided', () => {
    renderHook(() => useSteamGame(12345))
    expect(mockUseSWR).toHaveBeenCalledWith('/games/12345', expect.any(Function))
  })

  it('passes null as key when gameId is 0', () => {
    renderHook(() => useSteamGame(0))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('fetcher calls client.getSteamGame', async () => {
    const mockClient = { getSteamGame: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useSteamGame(42))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteamGame).toHaveBeenCalledWith({ gameId: 42 })
  })
})

describe('useSteamDistribution', () => {
  it('uses /games/distribution/${bucket} as key', () => {
    renderHook(() => useSteamDistribution(10))
    expect(mockUseSWR).toHaveBeenCalledWith('/games/distribution/10', expect.any(Function))
  })

  it('fetcher calls client.getSteamDistribution', async () => {
    const mockClient = { getSteamDistribution: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useSteamDistribution(10))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteamDistribution).toHaveBeenCalledWith({ bucket: 10 })
  })
})

describe('useSteamProgress', () => {
  it('uses correct key with dates', () => {
    renderHook(() => useSteamProgress('2024-01-01', '2024-12-31'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/games/progress', '2024-01-01', '2024-12-31'])
  })

  it('uses key with undefined dates', () => {
    renderHook(() => useSteamProgress())
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/games/progress', undefined, undefined])
  })

  it('fetcher calls client.getSteam with dates', async () => {
    const mockClient = { getSteam: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useSteamProgress('2024-01-01', '2024-12-31'))
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getSteam).toHaveBeenCalledWith({
      dateStart: '2024-01-01',
      dateEnd: '2024-12-31'
    })
  })
})

describe('useRefreshSteam', () => {
  it('fetches the steam refresh endpoint with credentials', async () => {
    global.fetch = jest.fn().mockResolvedValue({})
    const { result } = renderHook(() => useRefreshSteam())
    await result.current()
    expect(global.fetch).toHaveBeenCalledWith('https://api.test/games/api/progress/steam/refresh', {
      credentials: 'include'
    })
  })
})

describe('useIntegrations', () => {
  it('uses /games/integrations as key', () => {
    renderHook(() => useIntegrations())
    expect(mockUseSWR).toHaveBeenCalledWith(
      '/games/integrations',
      expect.any(Function),
      expect.objectContaining({ revalidateOnFocus: false })
    )
  })

  it('fetcher calls client.getIntegrations', async () => {
    const mockClient = { getIntegrations: jest.fn().mockResolvedValue({}) }
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce(mockClient)
    renderHook(() => useIntegrations())
    const fetcher = mockUseSWR.mock.calls[0]![1]!
    await fetcher()
    expect(mockClient.getIntegrations).toHaveBeenCalledWith({})
  })
})

describe('useSaveIntegrations', () => {
  it('calls client.saveIntegrations with the integrations payload', async () => {
    const mockSave = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValueOnce({ saveIntegrations: mockSave })
    const { result } = renderHook(() => useSaveIntegrations())
    const integrations = { steamUserId: '123' }
    // @ts-expect-error -- partial Integrations message is enough for the test
    await result.current(integrations)
    expect(mockSave).toHaveBeenCalledWith({ integrations })
  })
})
