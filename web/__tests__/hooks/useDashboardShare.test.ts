import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))

const clientMocks = {
  getDashboardShare: jest.fn().mockResolvedValue({}),
  createDashboardShare: jest.fn().mockResolvedValue({ share: { token: 'tok' } }),
  deleteDashboardShare: jest.fn().mockResolvedValue({}),
  getSharedLibrary: jest.fn().mockResolvedValue({}),
  getSharedBooksProgress: jest.fn().mockResolvedValue({}),
  getSharedFeedsSummary: jest.fn().mockResolvedValue({}),
  getSharedSteam: jest.fn().mockResolvedValue({}),
  getSharedSteamGame: jest.fn().mockResolvedValue({}),
  getSharedRecentlyActiveGames: jest.fn().mockResolvedValue({})
}

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => clientMocks)
}))
jest.mock('@/lib/gen/dashboard/v1/dashboard_pb', () => ({
  DashboardService: {},
  DashboardKind: { UNSPECIFIED: 0, GAMES: 1, READING: 2 }
}))
jest.mock('@/lib/gen/dashboard/v1/reading_pb', () => ({ PublicReadingDashboardService: {} }))
jest.mock('@/lib/gen/dashboard/v1/games_pb', () => ({ PublicGamesDashboardService: {} }))

import useSWR from 'swr'
import {
  useDashboardShare,
  useCreateDashboardShare,
  useDeleteDashboardShare,
  useSharedLibrary,
  useSharedBooksProgress,
  useSharedFeedsSummary,
  useSharedSteam,
  useSharedSteamProgress,
  useSharedSteamGame,
  useSharedRecentlyActiveGames
} from '@/hooks/useDashboardShare'

const mockUseSWR = jest.mocked(useSWR)

describe('useDashboardShare hooks', () => {
  beforeEach(() => {
    mockUseSWR.mockReset()
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValue({ data: undefined })
    jest.clearAllMocks()
  })

  it('useDashboardShare queries the dashboard-scoped share key', async () => {
    renderHook(() => useDashboardShare('reading'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/dashboard/share/reading')
    await fetcher!()
    expect(clientMocks.getDashboardShare).toHaveBeenCalledWith({ kind: 2 })
  })

  it('useDashboardShare keys games independently from reading', () => {
    renderHook(() => useDashboardShare('games'))
    const [key] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/dashboard/share/games')
  })

  it('useCreateDashboardShare calls the RPC with the kind', async () => {
    const { result } = renderHook(() => useCreateDashboardShare('games'))
    await result.current()
    expect(clientMocks.createDashboardShare).toHaveBeenCalledWith({ kind: 1 })
  })

  it('useDeleteDashboardShare calls the RPC with the kind', async () => {
    const { result } = renderHook(() => useDeleteDashboardShare('reading'))
    await result.current()
    expect(clientMocks.deleteDashboardShare).toHaveBeenCalledWith({ kind: 2 })
  })

  it('useSharedLibrary keys by token and passes it to the RPC', async () => {
    renderHook(() => useSharedLibrary('tok-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/dashboard/reading/tok-1')
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

  it('useSharedFeedsSummary keys by token and passes it to the RPC', async () => {
    renderHook(() => useSharedFeedsSummary('tok-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/dashboard/reading/tok-1/feeds-summary')
    await fetcher!()
    expect(clientMocks.getSharedFeedsSummary).toHaveBeenCalledWith({ token: 'tok-1' })
  })

  it('useSharedFeedsSummary is disabled without a token', () => {
    renderHook(() => useSharedFeedsSummary(''))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()
  })

  it('useSharedBooksProgress is disabled without a token', () => {
    renderHook(() => useSharedBooksProgress(''))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()
  })

  it('useSharedSteam keys by token', async () => {
    renderHook(() => useSharedSteam('tok-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/dashboard/games/tok-1')
    await fetcher!()
    expect(clientMocks.getSharedSteam).toHaveBeenCalledWith({ token: 'tok-1' })
  })

  it('useSharedSteam is disabled without a token', () => {
    renderHook(() => useSharedSteam(''))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()
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

  it('useSharedSteamProgress is disabled without a token', () => {
    renderHook(() => useSharedSteamProgress(''))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()
  })

  it('useSharedSteamGame keys by token and game id', async () => {
    renderHook(() => useSharedSteamGame('tok-1', 7))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/dashboard/games/tok-1/7')
    await fetcher!()
    expect(clientMocks.getSharedSteamGame).toHaveBeenCalledWith({ token: 'tok-1', gameId: 7 })
  })

  it('useSharedSteamGame is disabled without a token or game id', () => {
    renderHook(() => useSharedSteamGame('', 0))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()
  })

  it('useSharedRecentlyActiveGames keys by token', async () => {
    renderHook(() => useSharedRecentlyActiveGames('tok-1'))
    const [key, fetcher] = mockUseSWR.mock.calls[0]!
    expect(key).toBe('/dashboard/games/tok-1/recent')
    await fetcher!()
    expect(clientMocks.getSharedRecentlyActiveGames).toHaveBeenCalledWith({ token: 'tok-1' })
  })

  it('useSharedRecentlyActiveGames is disabled without a token', () => {
    renderHook(() => useSharedRecentlyActiveGames(''))
    expect(mockUseSWR.mock.calls[0]![0]).toBeNull()
  })
})
