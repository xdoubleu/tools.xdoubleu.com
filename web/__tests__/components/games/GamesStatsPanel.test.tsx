import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

jest.mock('@/hooks/useGames', () => ({
  useSteam: jest.fn(),
  useSteamProgress: jest.fn()
}))

jest.mock('@/components/games/SteamDistributionChart', () => {
  return function MockSteamDistributionChart() {
    return <div data-testid="distribution-chart" />
  }
})

jest.mock('@/components/games/SteamProgressChart', () => {
  return function MockSteamProgressChart() {
    return <div data-testid="progress-chart" />
  }
})

import GamesStatsPanel from '@/components/games/GamesStatsPanel'
import { useSteam, useSteamProgress } from '@/hooks/useGames'
import { create } from '@bufbuild/protobuf'
import { GetSteamResponseSchema, SteamResponseSchema } from '@/lib/gen/games/v1/games_pb'

const mockUseSteam = jest.mocked(useSteam)
const mockUseSteamProgress = jest.mocked(useSteamProgress)

function mockSteam() {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSteam.mockReturnValue({
    data: create(GetSteamResponseSchema, {
      steam: create(SteamResponseSchema, {
        totalBacklog: 2,
        currentRate: '50.00',
        distribution: [1, 2, 3]
      })
    }),
    error: undefined,
    isLoading: false
  })
}

function mockProgress(data: { labels: string[]; values: string[] } = { labels: [], values: [] }) {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSteamProgress.mockReturnValue({
    data: create(GetSteamResponseSchema, { steam: create(SteamResponseSchema, data) }),
    isLoading: false
  })
}

describe('GamesStatsPanel', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('does not fetch or render stats until opened', () => {
    mockSteam()
    mockProgress()
    render(<GamesStatsPanel />)
    expect(mockUseSteam).not.toHaveBeenCalled()
    expect(screen.queryByTestId('distribution-chart')).not.toBeInTheDocument()
  })

  it('renders stats and both charts when opened', () => {
    mockSteam()
    mockProgress({ labels: ['Jan'], values: ['10'] })
    render(<GamesStatsPanel />)

    fireEvent.click(screen.getByRole('button', { name: 'Open library stats' }))

    expect(screen.getByText('Total backlog')).toBeInTheDocument()
    expect(screen.getByText('50.00%')).toBeInTheDocument()
    expect(screen.getByTestId('distribution-chart')).toBeInTheDocument()
    expect(screen.getByTestId('progress-chart')).toBeInTheDocument()
  })

  it('shows an empty progress message when there is no data in range', () => {
    mockSteam()
    mockProgress()
    render(<GamesStatsPanel />)
    fireEvent.click(screen.getByRole('button', { name: 'Open library stats' }))
    expect(screen.getByText('No progress data for this range.')).toBeInTheDocument()
  })

  it('closes via the close chevron', () => {
    mockSteam()
    mockProgress()
    render(<GamesStatsPanel />)
    fireEvent.click(screen.getByRole('button', { name: 'Open library stats' }))
    expect(screen.getByText('Total backlog')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Close library stats' }))
    expect(screen.queryByText('Total backlog')).not.toBeInTheDocument()
  })
})
