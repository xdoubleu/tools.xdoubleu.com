import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockClient = {
  getFeedInfo: jest.fn().mockResolvedValue({ feedVersion: '2026-08-31' }),
  searchStations: jest.fn().mockResolvedValue({ stations: [{ stopId: 'SA', name: 'Alpha' }] }),
  searchJourneys: jest.fn().mockResolvedValue({ journeys: [] }),
  getJourneyDetail: jest.fn().mockResolvedValue({ journey: { legs: [] } }),
  listSavedCommutes: jest.fn().mockResolvedValue({ savedCommutes: [] }),
  createSavedCommute: jest.fn().mockResolvedValue({}),
  deleteSavedCommute: jest.fn().mockResolvedValue({})
}
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/trains/v1/trains_pb', () => ({
  TrainService: {}
}))

import useSWR from 'swr'
import {
  useTrainsFeedInfo,
  useStationSearch,
  useJourneySearch,
  useJourneyDetail,
  useSavedCommutes
} from '@/hooks/useTrains'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useTrainsFeedInfo', () => {
  it('uses the feed-info key', () => {
    renderHook(() => useTrainsFeedInfo())
    expect(mockUseSWR).toHaveBeenCalledWith('/trains/feed-info', expect.any(Function))
  })
})

describe('useStationSearch', () => {
  it('keys by the trimmed query', () => {
    renderHook(() => useStationSearch('  Brussels  '))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/trains/stations', 'Brussels'],
      expect.any(Function),
      { keepPreviousData: true }
    )
  })

  it('returns an empty station list when there is no data yet', () => {
    const { result } = renderHook(() => useStationSearch(''))
    expect(result.current.stations).toEqual([])
  })

  it('returns the fetched stations', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValue({
      data: { stations: [{ stopId: 'SA', name: 'Alpha' }] },
      isLoading: false
    })
    const { result } = renderHook(() => useStationSearch('a'))
    expect(result.current.stations).toEqual([{ stopId: 'SA', name: 'Alpha' }])
  })
})

describe('useJourneySearch', () => {
  it('passes null as key until both stations are chosen', () => {
    renderHook(() => useJourneySearch('', '', '2026-01-01T00:00:00Z', false))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })

  it('keys by the search criteria once both stations are chosen', () => {
    renderHook(() => useJourneySearch('SA', 'SB', '2026-01-01T00:00:00Z', true))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/trains/journeys', 'SA', 'SB', '2026-01-01T00:00:00Z', true],
      expect.any(Function)
    )
  })
})

describe('useSavedCommutes', () => {
  it('keys by the saved-commutes key and fetches the list', async () => {
    renderHook(() => useSavedCommutes())
    expect(mockUseSWR).toHaveBeenCalledWith('/trains/saved-commutes', expect.any(Function))
    const [, fetcher] = mockUseSWR.mock.calls[0]!
    await fetcher!()
    expect(mockClient.listSavedCommutes).toHaveBeenCalledWith({})
  })

  it('create posts the pair then revalidates', async () => {
    const mutate = jest.fn().mockResolvedValue(undefined)
    // @ts-expect-error -- partial SWRResponse
    mockUseSWR.mockReturnValue({ data: undefined, mutate })
    const { result } = renderHook(() => useSavedCommutes())
    await result.current.create('Label', 'SA', 'SB')
    expect(mockClient.createSavedCommute).toHaveBeenCalledWith({
      label: 'Label',
      originStopId: 'SA',
      destinationStopId: 'SB'
    })
    expect(mutate).toHaveBeenCalled()
  })

  it('remove deletes by id then revalidates', async () => {
    const mutate = jest.fn().mockResolvedValue(undefined)
    // @ts-expect-error -- partial SWRResponse
    mockUseSWR.mockReturnValue({ data: undefined, mutate })
    const { result } = renderHook(() => useSavedCommutes())
    await result.current.remove('c1')
    expect(mockClient.deleteSavedCommute).toHaveBeenCalledWith({ id: 'c1' })
    expect(mutate).toHaveBeenCalled()
  })
})

describe('useJourneyDetail', () => {
  it('passes null as key when there is no journey id', () => {
    renderHook(() => useJourneyDetail(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function), {
      revalidateOnFocus: false
    })
  })

  it('keys by the journey id', () => {
    renderHook(() => useJourneyDetail('journey-1'))
    expect(mockUseSWR).toHaveBeenCalledWith(
      ['/trains/journey', 'journey-1'],
      expect.any(Function),
      { revalidateOnFocus: false }
    )
  })

  it('fetches via getJourneyDetail', async () => {
    renderHook(() => useJourneyDetail('journey-1'))
    const [, fetcher] = mockUseSWR.mock.calls[0]!
    await fetcher!()
    expect(mockClient.getJourneyDetail).toHaveBeenCalledWith({ journeyId: 'journey-1' })
  })
})
