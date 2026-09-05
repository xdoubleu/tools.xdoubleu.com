import { render, screen, fireEvent } from '@testing-library/react'
import TrainsClient from '@/components/trains/TrainsClient'

const mockUseTrainsFeedInfo = jest.fn()
const mockUseStationSearch = jest.fn()
const mockUseJourneySearch = jest.fn()

jest.mock('@/hooks/useTrains', () => ({
  useTrainsFeedInfo: () => mockUseTrainsFeedInfo(),
  useStationSearch: (query: string) => mockUseStationSearch(query),
  useJourneySearch: (
    originStopId: string,
    destinationStopId: string,
    time: string,
    arriveBy: boolean
  ) => mockUseJourneySearch(originStopId, destinationStopId, time, arriveBy)
}))

beforeEach(() => {
  mockUseTrainsFeedInfo.mockReturnValue({ data: { feedVersion: '2026-08-31' } })
  mockUseStationSearch.mockReturnValue({
    stations: [
      { stopId: 'SA', name: 'Alpha' },
      { stopId: 'SB', name: 'Bravo' }
    ]
  })
  mockUseJourneySearch.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
})

describe('TrainsClient', () => {
  it('renders the heading and attribution', () => {
    render(<TrainsClient />)
    expect(screen.getByRole('heading', { name: 'Trains' })).toBeInTheDocument()
    expect(screen.getByText(/Source: NMBS-SNCB - Open Data - 2026-08-31/)).toBeInTheDocument()
  })

  it('does not search until both stations are chosen', () => {
    render(<TrainsClient />)
    expect(mockUseJourneySearch).toHaveBeenLastCalledWith('', '', expect.any(String), false)
  })

  it('searches once origin and destination are both picked', () => {
    render(<TrainsClient />)

    fireEvent.change(screen.getByLabelText('From'), { target: { value: 'Al' } })
    fireEvent.focus(screen.getByLabelText('From'))
    fireEvent.mouseDown(screen.getByText('Alpha'))

    fireEvent.change(screen.getByLabelText('To'), { target: { value: 'Br' } })
    fireEvent.focus(screen.getByLabelText('To'))
    fireEvent.mouseDown(screen.getByText('Bravo'))

    expect(mockUseJourneySearch).toHaveBeenLastCalledWith('SA', 'SB', expect.any(String), false)
  })

  it('swaps origin and destination', () => {
    render(<TrainsClient />)

    fireEvent.change(screen.getByLabelText('From'), { target: { value: 'Al' } })
    fireEvent.focus(screen.getByLabelText('From'))
    fireEvent.mouseDown(screen.getByText('Alpha'))

    fireEvent.click(screen.getByLabelText('Swap origin and destination'))

    expect(screen.getByLabelText('To')).toHaveValue('Alpha')
    expect(screen.getByLabelText('From')).toHaveValue('')
  })

  it('treats a cleared date as no request time', () => {
    render(<TrainsClient />)
    fireEvent.change(screen.getByLabelText('Date'), { target: { value: '' } })
    expect(mockUseJourneySearch).toHaveBeenLastCalledWith('', '', '', false)
  })

  it('defaults feedImported to true while feed info is still loading', () => {
    mockUseTrainsFeedInfo.mockReturnValue({ data: undefined })
    render(<TrainsClient />)
    expect(screen.queryByText(/hasn't been imported yet/)).not.toBeInTheDocument()
  })

  it('updates the search time when the time field changes', () => {
    render(<TrainsClient />)
    fireEvent.change(screen.getByLabelText('Time'), { target: { value: '14:30' } })
    expect(screen.getByLabelText('Time')).toHaveValue('14:30')
  })

  it('toggles between depart-at and arrive-by', () => {
    render(<TrainsClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Arrive by' }))
    expect(screen.getByRole('button', { name: 'Arrive by' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )

    fireEvent.click(screen.getByRole('button', { name: 'Depart at' }))
    expect(screen.getByRole('button', { name: 'Depart at' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })
})
