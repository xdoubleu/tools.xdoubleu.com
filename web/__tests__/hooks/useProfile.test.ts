import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))

const clientMocks = {
  getProfileShare: jest.fn().mockResolvedValue({}),
  createProfileShare: jest.fn().mockResolvedValue({ share: { token: 'tok' } }),
  deleteProfileShare: jest.fn().mockResolvedValue({}),
  getSharedLibrary: jest.fn().mockResolvedValue({}),
  getSharedBooksProgress: jest.fn().mockResolvedValue({}),
  getSharedSteam: jest.fn().mockResolvedValue({}),
  getSharedSteamGame: jest.fn().mockResolvedValue({}),
  getSharedRecentlyActiveGames: jest.fn().mockResolvedValue({})
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => clientMocks)
}))
jest.mock('@/lib/gen/profile/v1/profile_pb', () => ({ ProfileService: {} }))
jest.mock('@/lib/gen/books/v1/public_pb', () => ({ PublicLibraryService: {} }))
jest.mock('@/lib/gen/games/v1/public_pb', () => ({ PublicGamesService: {} }))

import useSWR from 'swr'
import {
  useProfileShare,
  useCreateProfileShare,
  useDeleteProfileShare,
  useSharedLibrary,
  useSharedBooksProgress,
  useSharedSteam,
  useSharedSteamProgress,
  useSharedSteamGame,
  useSharedRecentlyActiveGames
} from '@/hooks/useProfile'

const mockUseSWR = jest.mocked(useSWR)

describe('useProfile hooks', () => {
  beforeEach(() => {
    mockUseSWR.mockReset()
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValue({ data: undefined })
    jest.clearAllMocks()
  })

  it('useProfileShare queries the share key', async () => {
    renderHook(() => useProfileShare())
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/profile/share')
    await fetcher!()
    expect(clientMocks.getProfileShare).toHaveBeenCalled()
  })

  it('useCreateProfileShare calls the RPC', async () => {
    const { result } = renderHook(() => useCreateProfileShare())
    await result.current()
    expect(clientMocks.createProfileShare).toHaveBeenCalled()
  })

  it('useDeleteProfileShare calls the RPC', async () => {
    const { result } = renderHook(() => useDeleteProfileShare())
    await result.current()
    expect(clientMocks.deleteProfileShare).toHaveBeenCalled()
  })

  it('useSharedLibrary keys by token and passes it to the RPC', async () => {
    renderHook(() => useSharedLibrary('tok-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/profile/tok-1/books')
    await fetcher!()
    expect(clientMocks.getSharedLibrary).toHaveBeenCalledWith({ token: 'tok-1' })
  })

  it('useSharedLibrary is disabled without a token', () => {
    renderHook(() => useSharedLibrary(''))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()
  })

  it('useSharedBooksProgress passes the date range', async () => {
    renderHook(() => useSharedBooksProgress('tok-1', '2026-01-01', '2026-02-01'))
    const [, fetcher] = mockUseSWR.mock.calls[0]!
    await fetcher!()
    expect(clientMocks.getSharedBooksProgress).toHaveBeenCalledWith({
      token: 'tok-1',
      dateStart: '2026-01-01',
      dateEnd: '2026-02-01'
    })
  })

  it('useSharedSteam keys by token', async () => {
    renderHook(() => useSharedSteam('tok-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/profile/tok-1/games')
    await fetcher!()
    expect(clientMocks.getSharedSteam).toHaveBeenCalledWith({ token: 'tok-1' })
  })

  it('useSharedSteamProgress passes the date range', async () => {
    renderHook(() => useSharedSteamProgress('tok-1', '2026-01-01', '2026-02-01'))
    const [, fetcher] = mockUseSWR.mock.calls[0]!
    await fetcher!()
    expect(clientMocks.getSharedSteam).toHaveBeenCalledWith({
      token: 'tok-1',
      dateStart: '2026-01-01',
      dateEnd: '2026-02-01'
    })
  })

  it('useSharedSteamGame keys by token and game id', async () => {
    renderHook(() => useSharedSteamGame('tok-1', 7))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/profile/tok-1/games/7')
    await fetcher!()
    expect(clientMocks.getSharedSteamGame).toHaveBeenCalledWith({ token: 'tok-1', gameId: 7 })
  })

  it('useSharedRecentlyActiveGames keys by token', async () => {
    renderHook(() => useSharedRecentlyActiveGames('tok-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/profile/tok-1/games/recent')
    await fetcher!()
    expect(clientMocks.getSharedRecentlyActiveGames).toHaveBeenCalledWith({ token: 'tok-1' })
  })
})
