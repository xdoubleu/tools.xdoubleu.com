import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import JourneyAlternativePanel from '@/components/trains/JourneyAlternativePanel'
import {
  JourneyAlternativeSchema,
  JourneySchema,
  LegSchema,
  type Journey
} from '@/lib/gen/trains/v1/trains_pb'

const replanJourney: Journey = create(JourneySchema, {
  legs: [create(LegSchema, { tripShortName: 'IC1717', routeShortName: 'IC', headsign: 'Ghent' })],
  departureTime: '2026-06-01T15:50:00Z',
  arrivalTime: '2026-06-01T16:40:00Z',
  transfers: 0,
  journeyId: 'replan-id'
})

function alt(includeJourney = true) {
  return create(JourneyAlternativeSchema, {
    reason: "You'll miss the 17:42 at Mechelen by 4 min",
    fromStopName: 'Mechelen',
    journey: includeJourney ? replanJourney : undefined
  })
}

describe('JourneyAlternativePanel', () => {
  it('shows the reason, the alternative row and a switch action', () => {
    render(<JourneyAlternativePanel alternative={alt()} />)

    expect(screen.getByText(/miss the 17:42 at Mechelen/)).toBeInTheDocument()
    expect(screen.getByText(/Alternative from Mechelen/)).toBeInTheDocument()
    expect(screen.getByText(/IC1717/)).toBeInTheDocument()

    const link = screen.getByRole('link', { name: /switch to this journey/i })
    expect(link).toHaveAttribute('href', '/trains/replan-id')
  })

  it('still shows the reason when no alternative route was found', () => {
    render(<JourneyAlternativePanel alternative={alt(false)} />)

    expect(screen.getByText(/miss the 17:42 at Mechelen/)).toBeInTheDocument()
    expect(screen.getByText(/No alternative route found from Mechelen/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /switch to this journey/i })).not.toBeInTheDocument()
  })
})
