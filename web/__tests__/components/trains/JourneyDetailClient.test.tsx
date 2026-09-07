import { render, screen } from '@testing-library/react'

const mockMutate = jest.fn()
const mockUseJourneyDetail = jest.fn()
const mockUseJourneyLive = jest.fn()

jest.mock('@/hooks/useTrains', () => ({
  useJourneyDetail: (journeyId: string) => mockUseJourneyDetail(journeyId)
}))
jest.mock('@/lib/trains/journeySocket', () => ({
  useJourneyLive: (journeyId: string, refetch: () => void) => mockUseJourneyLive(journeyId, refetch)
}))

import JourneyDetailClient from '@/components/trains/JourneyDetailClient'

describe('JourneyDetailClient', () => {
  beforeEach(() => {
    mockMutate.mockClear()
    mockUseJourneyDetail.mockReset()
    mockUseJourneyLive.mockReset()
    mockUseJourneyLive.mockReturnValue({ connected: true, pushedDetail: null })
  })

  it('shows the loading state', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
      mutate: mockMutate
    })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows the error state', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: undefined,
      error: new Error('boom'),
      isLoading: false,
      mutate: mockMutate
    })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.getByText('Failed to load journey.')).toBeInTheDocument()
  })

  it('shows the error state when no journey data is available at all', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: false,
      mutate: mockMutate
    })
    mockUseJourneyLive.mockReturnValue({ connected: false, pushedDetail: null })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.getByText('Failed to load journey.')).toBeInTheDocument()
  })

  it('renders every leg from the fetched journey and a live indicator', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: {
        journey: {
          legs: [
            {
              tripShortName: 'IC900',
              routeShortName: 'IC',
              headsign: 'Echo',
              cancelled: false,
              stops: [],
              alerts: []
            }
          ]
        }
      },
      error: undefined,
      isLoading: false,
      mutate: mockMutate
    })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.getByText(/IC900/)).toBeInTheDocument()
    expect(screen.getByText('Live')).toBeInTheDocument()
  })

  it('shows "Reconnecting…" when the socket is not currently connected', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: { journey: { legs: [] } },
      error: undefined,
      isLoading: false,
      mutate: mockMutate
    })
    mockUseJourneyLive.mockReturnValue({ connected: false, pushedDetail: null })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.getByText('Reconnecting…')).toBeInTheDocument()
  })

  it('prefers a pushed live update over the last SWR fetch', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: {
        journey: {
          legs: [
            { tripShortName: 'STALE', routeShortName: '', headsign: '', stops: [], alerts: [] }
          ]
        }
      },
      error: undefined,
      isLoading: false,
      mutate: mockMutate
    })
    mockUseJourneyLive.mockReturnValue({
      connected: true,
      pushedDetail: {
        legs: [{ tripShortName: 'FRESH', routeShortName: '', headsign: '', stops: [], alerts: [] }]
      }
    })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.getByText(/FRESH/)).toBeInTheDocument()
    expect(screen.queryByText(/STALE/)).not.toBeInTheDocument()
  })
})
