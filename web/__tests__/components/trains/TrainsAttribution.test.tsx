import { render, screen } from '@testing-library/react'
import TrainsAttribution from '@/components/trains/TrainsAttribution'

const mockUseTrainsFeedInfo = jest.fn()
jest.mock('@/hooks/useTrains', () => ({
  useTrainsFeedInfo: () => mockUseTrainsFeedInfo()
}))

describe('TrainsAttribution', () => {
  it('includes the feed version when known', () => {
    mockUseTrainsFeedInfo.mockReturnValue({ data: { feedVersion: '2026-08-31' } })
    render(<TrainsAttribution />)
    expect(screen.getByText(/Source: NMBS-SNCB - Open Data - 2026-08-31/)).toBeInTheDocument()
    expect(screen.getByText(/modified by tools\.xdoubleu\.com/)).toBeInTheDocument()
  })

  it('omits the date when the feed version is not yet known', () => {
    mockUseTrainsFeedInfo.mockReturnValue({ data: undefined })
    render(<TrainsAttribution />)
    expect(
      screen.getByText(
        'Source: NMBS-SNCB - Open Data. Contains data originally published by NMBS-SNCB, modified by tools.xdoubleu.com.'
      )
    ).toBeInTheDocument()
  })
})
