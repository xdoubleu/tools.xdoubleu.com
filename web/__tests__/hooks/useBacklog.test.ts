import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
jest.mock('@/lib/gen/backlog/v1/books_connect', () => ({
  BooksService: {}
}))
jest.mock('@/lib/gen/backlog/v1/games_connect', () => ({
  GamesService: {}
}))
jest.mock('@/lib/gen/icsproxy/v1/proxy_connect', () => ({
  ICSProxyService: {}
}))

import useSWR from 'swr'
import {
  useBacklogLibrary,
  useBacklogSteam,
  useBacklogSteamGame,
  useBacklogDistribution,
  useBooksProgress
} from '@/hooks/useBacklog'
import { useICSFeeds, useICSPreview } from '@/hooks/useICSProxy'

const mockUseSWR = useSWR as jest.Mock

beforeEach(() => {
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useBacklogLibrary', () => {
  it('uses /backlog/books as key', () => {
    renderHook(() => useBacklogLibrary())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/books', expect.any(Function))
  })
})

describe('useBacklogSteam', () => {
  it('uses /backlog/steam as key', () => {
    renderHook(() => useBacklogSteam())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/steam', expect.any(Function))
  })
})

describe('useBacklogSteamGame', () => {
  it('uses correct key when gameId is provided', () => {
    renderHook(() => useBacklogSteamGame(12345))
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/steam/12345', expect.any(Function))
  })

  it('passes null as key when gameId is 0', () => {
    renderHook(() => useBacklogSteamGame(0))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useBacklogDistribution', () => {
  it('uses /backlog/steam/distribution as key', () => {
    renderHook(() => useBacklogDistribution())
    expect(mockUseSWR).toHaveBeenCalledWith('/backlog/steam/distribution', expect.any(Function))
  })
})

describe('useBooksProgress', () => {
  it('uses correct key with date range', () => {
    renderHook(() => useBooksProgress('2024-01-01', '2024-12-31'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toEqual(['/backlog/books/progress', '2024-01-01', '2024-12-31'])
  })

  it('passes null as key when no dates provided', () => {
    renderHook(() => useBooksProgress())
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('useICSFeeds', () => {
  it('uses /icsproxy as key', () => {
    renderHook(() => useICSFeeds())
    expect(mockUseSWR).toHaveBeenCalledWith('/icsproxy', expect.any(Function))
  })
})

describe('useICSPreview', () => {
  it('encodes the sourceUrl in the key when given', () => {
    renderHook(() => useICSPreview('https://cal.example.com/feed.ics'))
    const [key] = mockUseSWR.mock.calls[0]
    expect(key).toContain('/icsproxy/preview?url=')
    expect(key).toContain('cal.example.com')
  })

  it('passes null as key when sourceUrl is empty', () => {
    renderHook(() => useICSPreview(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})
