import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import JourneyResults from '@/components/trains/JourneyResults'
import { JourneySchema, LegSchema, type Journey } from '@/lib/gen/trains/v1/trains_pb'

function journey(transfers = 0): Journey {
  return create(JourneySchema, {
    legs: [create(LegSchema, { tripShortName: '515', routeShortName: 'IC' })],
    departureTime: '2026-01-01T08:00:00.000Z',
    arrivalTime: '2026-01-01T09:20:00.000Z',
    transfers
  })
}

describe('JourneyResults', () => {
  it('renders nothing when not ready', () => {
    const { container } = render(
      <JourneyResults
        ready={false}
        isLoading={false}
        error={undefined}
        feedImported
        journeys={[]}
      />
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('shows the loading state', () => {
    render(<JourneyResults ready isLoading error={undefined} feedImported journeys={[]} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows the error state', () => {
    render(
      <JourneyResults
        ready
        isLoading={false}
        error={new Error('boom')}
        feedImported
        journeys={[]}
      />
    )
    expect(screen.getByText('Failed to load journeys.')).toBeInTheDocument()
  })

  it('shows the feed-not-imported state', () => {
    render(
      <JourneyResults
        ready
        isLoading={false}
        error={undefined}
        feedImported={false}
        journeys={[]}
      />
    )
    expect(screen.getByText(/hasn't been imported yet/)).toBeInTheDocument()
  })

  it('shows the no-service state', () => {
    render(<JourneyResults ready isLoading={false} error={undefined} feedImported journeys={[]} />)
    expect(screen.getByText(/No trains found/)).toBeInTheDocument()
  })

  it('renders a direct journey row', () => {
    render(
      <JourneyResults
        ready
        isLoading={false}
        error={undefined}
        feedImported
        journeys={[journey()]}
      />
    )
    expect(screen.getByText('Direct · 515')).toBeInTheDocument()
    expect(screen.getByText('1h 20m')).toBeInTheDocument()
  })

  it('renders a journey with changes', () => {
    render(
      <JourneyResults
        ready
        isLoading={false}
        error={undefined}
        feedImported
        journeys={[journey(2)]}
      />
    )
    expect(screen.getByText('2 changes · 515')).toBeInTheDocument()
  })
})
