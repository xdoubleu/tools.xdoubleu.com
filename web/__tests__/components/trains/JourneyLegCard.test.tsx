import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import JourneyLegCard from '@/components/trains/JourneyLegCard'
import { LegDetailSchema, StopCallSchema, AlertSchema } from '@/lib/gen/trains/v1/trains_pb'

describe('JourneyLegCard', () => {
  it('renders every stop with its own status, not collapsing unknown into on-time', () => {
    const leg = create(LegDetailSchema, {
      tripShortName: 'IC900',
      routeShortName: 'IC',
      headsign: 'Echo',
      cancelled: false,
      stops: [
        create(StopCallSchema, {
          stopId: 'A1',
          stopName: 'Delta',
          status: 'on_time',
          // Origin stop — no scheduledArrival, only scheduledDeparture.
          scheduledDeparture: '2026-01-01T08:00:00.000Z',
          platform: '3',
          isBoardStop: true
        }),
        create(StopCallSchema, {
          stopId: 'M1',
          stopName: 'Midway',
          status: 'unknown'
        }),
        create(StopCallSchema, {
          stopId: 'B1',
          stopName: 'Echo',
          status: 'delayed',
          delaySeconds: 300,
          isAlightStop: true
        })
      ],
      alerts: []
    })

    render(<JourneyLegCard leg={leg} />)

    expect(screen.getByText('Delta')).toBeInTheDocument()
    expect(screen.getByText('Midway')).toBeInTheDocument()
    expect(screen.getByText('Echo', { selector: 'p' })).toBeInTheDocument()
    expect(screen.getByText('On time')).toBeInTheDocument()
    expect(screen.getByText('No live data')).toBeInTheDocument()
    expect(screen.getByText('Delayed 5m')).toBeInTheDocument()
  })

  it('shows a prominent cancellation banner for a cancelled leg', () => {
    const leg = create(LegDetailSchema, {
      tripShortName: 'IC900',
      routeShortName: 'IC',
      headsign: 'Echo',
      cancelled: true,
      stops: [],
      alerts: []
    })

    render(<JourneyLegCard leg={leg} />)
    expect(screen.getByText('Cancelled')).toBeInTheDocument()
  })

  it('renders attached alerts', () => {
    const leg = create(LegDetailSchema, {
      tripShortName: 'IC900',
      routeShortName: 'IC',
      headsign: 'Echo',
      cancelled: false,
      stops: [],
      alerts: [
        create(AlertSchema, {
          id: 'a1',
          headerText: 'Signal failure',
          descriptionText: 'Delays expected'
        })
      ]
    })

    render(<JourneyLegCard leg={leg} />)
    expect(screen.getByText('Signal failure')).toBeInTheDocument()
    expect(screen.getByText('Delays expected')).toBeInTheDocument()
  })
})
