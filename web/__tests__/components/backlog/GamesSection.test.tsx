import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'

const mockPush = jest.fn()
const mockRefreshSteam = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogSteam: jest.fn(),
  useSteamProgress: jest.fn(),
  useRefreshSteam: () => mockRefreshSteam
}))

jest.mock('swr', () => ({ mutate: jest.fn() }))
import { mutate } from 'swr'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush })
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/backlog/SteamDistributionChart', () => {
  return function MockSteamDistributionChart({
    onBucketClick
  }: {
    onBucketClick: (bucket: number) => void
  }) {
    return (
      <button data-testid="distribution-chart" onClick={() => onBucketClick(3)}>
        chart
      </button>
    )
  }
})

import GamesSection from '@/components/backlog/GamesSection'
import { useBacklogSteam, useSteamProgress } from '@/hooks/useBacklog'
import { create } from '@bufbuild/protobuf'
import { GameSchema } from '@/lib/gen/backlog/v1/games_pb'
import { SteamResponseSchema, GetSteamResponseSchema } from '@/lib/gen/backlog/v1/games_pb'

const mockUseBacklogSteam = jest.mocked(useBacklogSteam)
const mockUseSteamProgress = jest.mocked(useSteamProgress)

const inProgressGame = create(GameSchema, {
  id: 10,
  name: 'Hades',
  playtime: 1200,
  completionRate: '60%'
})

const notStartedGame = create(GameSchema, {
  id: 11,
  name: 'Celeste',
  playtime: 0,
  completionRate: '0%'
})

function mockSteam() {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogSteam.mockReturnValue({
    data: create(GetSteamResponseSchema, {
      steam: create(SteamResponseSchema, {
        inProgress: [inProgressGame],
        notStarted: [notStartedGame],
        totalBacklog: 2,
        currentRate: '1/week',
        distribution: [1, 2, 3]
      })
    }),
    error: undefined,
    isLoading: false
  })
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSteamProgress.mockReturnValue({ data: undefined, isLoading: false })
}

describe('GamesSection', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('renders the backlog by default', () => {
    mockSteam()
    render(<GamesSection />)
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.getByText('Celeste')).toBeInTheDocument()
    expect(screen.getByText('In Progress (1)')).toBeInTheDocument()
    expect(screen.getByText('Not Started (1)')).toBeInTheDocument()
  })

  it('filters games by the always-visible search', () => {
    mockSteam()
    render(<GamesSection />)
    fireEvent.change(screen.getByPlaceholderText('Search games...'), {
      target: { value: 'hades' }
    })
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.queryByText('Celeste')).not.toBeInTheDocument()
    expect(screen.getByText('In Progress (1)')).toBeInTheDocument()
    expect(screen.queryByText('Not Started (1)')).not.toBeInTheDocument()
  })

  it('shows a no-match message when the search excludes every game', () => {
    mockSteam()
    render(<GamesSection />)
    fireEvent.change(screen.getByPlaceholderText('Search games...'), {
      target: { value: 'nonexistent' }
    })
    expect(screen.getByText('No games match your search.')).toBeInTheDocument()
  })

  it('triggers a refresh and re-fetches the steam data', async () => {
    mockSteam()
    mockRefreshSteam.mockResolvedValue(undefined)
    render(<GamesSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    await waitFor(() => {
      expect(mockRefreshSteam).toHaveBeenCalled()
      expect(mutate).toHaveBeenCalledWith('/backlog/steam')
    })
  })

  it('links a game card to its detail page', () => {
    mockSteam()
    render(<GamesSection />)
    expect(screen.getByText('Hades').closest('a')).toHaveAttribute('href', '/backlog/steam/10')
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSteamProgress.mockReturnValue({ data: undefined, isLoading: false })
    render(<GamesSection />)
    expect(screen.getByText('Loading Steam library...')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({
      data: undefined,
      error: new Error('boom'),
      isLoading: false
    })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSteamProgress.mockReturnValue({ data: undefined, isLoading: false })
    render(<GamesSection />)
    expect(screen.getByText('Failed to load Steam data.')).toBeInTheDocument()
  })

  it('shows an empty progress message on the progress tab', () => {
    mockSteam()
    render(<GamesSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Progress' }))
    expect(screen.getByText('No progress data for this range.')).toBeInTheDocument()
  })

  it('renders the progress chart and updates the date range', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({ data: undefined, error: undefined, isLoading: false })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSteamProgress.mockReturnValue({
      data: create(GetSteamResponseSchema, {
        steam: create(SteamResponseSchema, {
          labels: ['Jan', 'Feb'],
          values: ['10', '20']
        })
      }),
      isLoading: false
    })
    render(<GamesSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Progress' }))
    expect(screen.queryByText('No progress data for this range.')).not.toBeInTheDocument()

    const from = screen.getByLabelText('From')
    fireEvent.change(from, { target: { value: '2026-01-01' } })
    expect(from).toHaveValue('2026-01-01')
  })

  it('navigates to a distribution bucket when a bar is clicked', () => {
    mockSteam()
    render(<GamesSection />)
    fireEvent.click(screen.getByRole('button', { name: 'Distribution' }))
    fireEvent.click(screen.getByTestId('distribution-chart'))
    expect(mockPush).toHaveBeenCalledWith('/backlog/steam/distribution/3')
  })
})
