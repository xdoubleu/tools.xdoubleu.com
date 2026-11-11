import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

const mockPush = vi.fn()
const mockRefresh = vi.fn()

vi.mock('@/hooks/useGames', () => ({
  useSteam: vi.fn(),
  useSteamProgress: vi.fn(),
  useRecentlyActiveGames: vi.fn()
}))

vi.mock('@/lib/games/steamRefresh', () => ({
  useSteamRefresh: vi.fn()
}))

vi.mock('swr', () => ({ mutate: vi.fn() }))
import { mutate } from 'swr'

vi.mock('@/hooks/navigation', () => ({
  useRouter: () => ({ push: mockPush })
}))

vi.mock('@/components/dashboard/DashboardShareButton', () => {
  return {
    default: function MockDashboardShareButton() {
      return <div data-testid="profile-share-button" />
    }
  }
})

vi.mock('@/components/games/SteamDistributionChart', () => {
  return {
    default: function MockSteamDistributionChart({
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
  }
})

import GamesDashboard from '@/components/dashboard/GamesDashboard'
import { useSteam, useSteamProgress, useRecentlyActiveGames } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  RecentGameSchema,
  SteamResponseSchema,
  GetSteamResponseSchema,
  GetRecentlyActiveGamesResponseSchema
} from '@/lib/gen/games/v1/games_pb'

const mockUseBacklogSteam = vi.mocked(useSteam)
const mockUseSteamProgress = vi.mocked(useSteamProgress)
const mockUseRecentlyActiveGames = vi.mocked(useRecentlyActiveGames)
const mockUseSteamRefresh = vi.mocked(useSteamRefresh)

function mockRefreshState(overrides: Partial<ReturnType<typeof useSteamRefresh>> = {}) {
  mockUseSteamRefresh.mockReturnValue({
    connected: true,
    isRefreshing: false,
    lastRefresh: null,
    refresh: mockRefresh,
    ...overrides
  })
}

function mockSteam() {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogSteam.mockReturnValue({
    data: create(GetSteamResponseSchema, {
      steam: create(SteamResponseSchema, {
        inProgress: [create(GameSchema, { id: 10, name: 'Hades' })],
        completed: [create(GameSchema, { id: 12, name: 'Bastion' })],
        totalBacklog: 2,
        currentRate: '50.00',
        distribution: [1, 2, 3]
      })
    }),
    error: undefined,
    isLoading: false
  })
}

function mockRecent(
  games = [
    create(RecentGameSchema, {
      id: 10,
      name: 'Hades',
      completionRate: '60.00',
      playtime: 120,
      lastPlayedAt: '2026-06-01',
      imageUrl: 'https://example.com/hades.jpg'
    })
  ]
) {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseRecentlyActiveGames.mockReturnValue({
    data: create(GetRecentlyActiveGamesResponseSchema, { games }),
    isLoading: false
  })
}

function mockNoProgress() {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSteamProgress.mockReturnValue({ data: undefined, isLoading: false })
}

describe('GamesDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockRefreshState()
  })

  it('renders the stat cards', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByText('Total backlog')).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(screen.getByText('In progress')).toBeInTheDocument()
    expect(screen.getByText('Completed')).toBeInTheDocument()
  })

  it('renders the completion rate with a percentage sign', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByText('Completion: 60%')).toBeInTheDocument()
  })

  it('renders recently active games linking to their detail page', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    const card = screen.getAllByText('Hades')[0]!
    expect(card.closest('a')).toHaveAttribute('href', '/games/10')
    expect(screen.getByText(/Last played 1 Jun/)).toBeInTheDocument()
  })

  it('renders the recent game icon with a locked square aspect ratio', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByAltText('Hades')).toHaveClass('h-8', 'w-8', 'object-cover')
  })

  it('shows the playtime for a recently played game', () => {
    mockSteam()
    mockRecent([
      create(RecentGameSchema, {
        id: 11,
        name: 'Celeste',
        completionRate: '10.00',
        playtime: 90,
        lastPlayedAt: '2026-06-02'
      })
    ])
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByText(/2 hrs/)).toBeInTheDocument()
  })

  it('shows an empty message when there is no recent activity', () => {
    mockSteam()
    mockRecent([])
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByText('No recently played games.')).toBeInTheDocument()
  })

  it('renders a clickable favourites stat card linking to the library', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({
      data: create(GetSteamResponseSchema, {
        steam: create(SteamResponseSchema, {
          inProgress: [create(GameSchema, { id: 10, name: 'Hades', favourite: true })],
          completed: [create(GameSchema, { id: 12, name: 'Bastion' })],
          totalBacklog: 2,
          currentRate: '50.00',
          distribution: [1, 2, 3]
        })
      }),
      error: undefined,
      isLoading: false
    })
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    const favouritesCard = screen.getByText('Favourites').closest('a')
    expect(favouritesCard).toHaveAttribute('href', '/games/library')
    expect(favouritesCard).toHaveTextContent('1')
  })

  it('shows an empty progress message', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    fireEvent.click(screen.getByRole('tab', { name: 'Progress' }))
    expect(screen.getByText('No progress data for this range.')).toBeInTheDocument()
  })

  it('renders the progress chart and updates the date range', () => {
    mockSteam()
    mockRecent()
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
    render(<GamesDashboard />)
    fireEvent.click(screen.getByRole('tab', { name: 'Progress' }))
    expect(screen.queryByText('No progress data for this range.')).not.toBeInTheDocument()
    const from = screen.getByLabelText('From')
    fireEvent.change(from, { target: { value: '2026-01-01' } })
    expect(from).toHaveValue('2026-01-01')
  })

  it('navigates to a distribution bucket when a bar is clicked', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    fireEvent.click(screen.getByTestId('distribution-chart'))
    expect(mockPush).toHaveBeenCalledWith('/games/distribution/3')
  })

  it('switches between the distribution and progress views', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)

    expect(screen.getByTestId('distribution-chart')).toBeInTheDocument()
    expect(screen.queryByText('No progress data for this range.')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Progress' }))
    expect(screen.getByText('No progress data for this range.')).toBeInTheDocument()
    expect(screen.queryByTestId('distribution-chart')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Distribution' }))
    expect(screen.getByTestId('distribution-chart')).toBeInTheDocument()
  })

  it('gives both chart views an explicit mobile height so they are visible when the grid is single-column', () => {
    mockSteam()
    mockRecent()
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSteamProgress.mockReturnValue({
      data: create(GetSteamResponseSchema, {
        steam: create(SteamResponseSchema, { labels: ['Jan'], values: ['10'] })
      }),
      isLoading: false
    })
    render(<GamesDashboard />)

    // Fixed height on mobile; fills its flex parent at lg.
    const distWrapper = screen.getByTestId('distribution-chart').parentElement!
    expect(distWrapper).toHaveClass('h-72', 'lg:h-full', 'lg:flex-1')

    fireEvent.click(screen.getByRole('tab', { name: 'Progress' }))
    const progressWrapper = document.querySelector('.h-72.lg\\:flex-1')
    expect(progressWrapper).toBeInTheDocument()
  })

  it('links to the full library', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByText('Browse full library').closest('a')).toHaveAttribute(
      'href',
      '/games/library'
    )
  })

  it('triggers a refresh and re-fetches both keys when a sync completes', () => {
    mockSteam()
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(mockRefresh).toHaveBeenCalled()
    const onSynced = mockUseSteamRefresh.mock.calls[0]![0]
    onSynced?.()
    expect(mutate).toHaveBeenCalledWith('/games')
    expect(mutate).toHaveBeenCalledWith('/games/recent')
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByText('Loading dashboard…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({
      data: undefined,
      error: new Error('boom'),
      isLoading: false
    })
    mockRecent()
    mockNoProgress()
    render(<GamesDashboard />)
    expect(screen.getByText('Failed to load Steam data.')).toBeInTheDocument()
  })
})
