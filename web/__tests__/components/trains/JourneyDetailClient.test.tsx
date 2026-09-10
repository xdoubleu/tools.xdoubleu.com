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

  it('surfaces an inline alternative when the journey is disrupted', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: {
        journey: {
          legs: [
            { tripShortName: 'IC900', routeShortName: 'IC', headsign: 'E', stops: [], alerts: [] }
          ],
          alternative: {
            reason: 'IC900 is cancelled',
            fromStopName: 'Mechelen',
            journey: {
              legs: [{ tripShortName: 'IC901', routeShortName: 'IC', headsign: 'E' }],
              departureTime: '2026-06-01T15:00:00Z',
              arrivalTime: '2026-06-01T15:40:00Z',
              transfers: 0,
              journeyId: 'alt-1'
            }
          }
        }
      },
      error: undefined,
      isLoading: false,
      mutate: mockMutate
    })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.getByText('IC900 is cancelled')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /switch to this journey/i })).toHaveAttribute(
      'href',
      '/trains/alt-1'
    )
  })

  it('renders no alternative panel for a healthy journey', () => {
    mockUseJourneyDetail.mockReturnValue({
      data: { journey: { legs: [], alternative: undefined } },
      error: undefined,
      isLoading: false,
      mutate: mockMutate
    })
    render(<JourneyDetailClient journeyId="journey-1" />)
    expect(screen.queryByText('Journey disrupted')).not.toBeInTheDocument()
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
