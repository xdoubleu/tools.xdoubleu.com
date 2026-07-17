import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { SteamResponseSchema, GameSchema, RecentGameSchema } from '@/lib/gen/games/v1/games_pb'
import {
  GetSharedSteamResponseSchema,
  GetSharedRecentlyActiveGamesResponseSchema
} from '@/lib/gen/games/v1/public_pb'

const mockUseSharedSteam = jest.fn()
const mockUseSharedSteamProgress = jest.fn()
const mockUseSharedRecentlyActiveGames = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useSharedSteam: () => mockUseSharedSteam(),
  useSharedSteamProgress: () => mockUseSharedSteamProgress(),
  useSharedRecentlyActiveGames: () => mockUseSharedRecentlyActiveGames()
}))

jest.mock('@/components/games/SteamDistributionChart', () => () => (
  <div data-testid="distribution-chart" />
))
jest.mock('@/components/games/SteamProgressChart', () => () => <div data-testid="progress-chart" />)

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import ProfileGamesClient from '@/components/profile/ProfileGamesClient'

function makeSteam() {
  return create(GetSharedSteamResponseSchema, {
    steam: create(SteamResponseSchema, {
      notStarted: [create(GameSchema, { id: 1, name: 'Backlog Game' })],
      inProgress: [
        create(GameSchema, { id: 2, name: 'Fav Game', favourite: true, completionRate: '50.00' })
      ],
      completed: [create(GameSchema, { id: 3, name: 'Done Game', completionRate: '100.00' })],
      totalBacklog: 2,
      currentRate: '42.00',
      distribution: [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1]
    }),
    lastSyncedAt: '2026-07-01T10:00:00Z'
  })
}

function makeRecent() {
  return create(GetSharedRecentlyActiveGamesResponseSchema, {
    games: [
      create(RecentGameSchema, {
        id: 2,
        name: 'Fav Game',
        completionRate: '50.00',
        recentUnlocks: 3,
        lastUnlockedAt: '2026-07-01'
      })
    ]
  })
}

describe('ProfileGamesClient', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUseSharedSteamProgress.mockReturnValue({ data: undefined, isLoading: false })
    mockUseSharedRecentlyActiveGames.mockReturnValue({ data: makeRecent() })
  })

  it('renders stat cards and last synced state', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesClient token="tok-1" />)

    expect(screen.getByText('Total backlog')).toBeInTheDocument()
    expect(screen.getByText('42.00%')).toBeInTheDocument()
    expect(screen.getByText(/Last synced:/)).toBeInTheDocument()
  })

  it('groups games with a Favourites section first', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesClient token="tok-1" />)

    expect(screen.getByRole('heading', { name: 'Favourites (1)' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'In Progress (1)' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Not Started (1)' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Completed (1)' })).toBeInTheDocument()
  })

  it('links game cards to the public game pages', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesClient token="tok-1" />)

    const links = screen.getAllByRole('link')
    expect(links.some((l) => l.getAttribute('href') === '/profile/games/tok-1/2')).toBe(true)
    expect(links.every((l) => l.getAttribute('href')?.startsWith('/profile/games/tok-1/'))).toBe(
      true
    )
  })

  it('is read-only: no refresh button', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesClient token="tok-1" />)

    expect(screen.queryByRole('button', { name: /refresh/i })).not.toBeInTheDocument()
  })

  it('shows an error state when steam data fails to load', () => {
    mockUseSharedSteam.mockReturnValue({ data: undefined, error: new Error('nope') })
    render(<ProfileGamesClient token="tok-1" />)

    expect(screen.getByText('Failed to load games.')).toBeInTheDocument()
  })

  it('filters games with the search input', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesClient token="tok-1" />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'backlog' }
    })
    expect(screen.getByRole('heading', { name: 'Not Started (1)' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'In Progress (1)' })).not.toBeInTheDocument()

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'zzz-no-match' }
    })
    expect(screen.getByText('No games match your search.')).toBeInTheDocument()
  })

  it('switches to the progress chart with a date range', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    mockUseSharedSteamProgress.mockReturnValue({
      data: {
        steam: {
          labels: ['2026-01-01', '2026-01-02'],
          values: ['1.5', '2.5']
        }
      },
      isLoading: false
    })
    render(<ProfileGamesClient token="tok-1" />)

    fireEvent.click(screen.getByRole('tab', { name: 'Progress' }))

    expect(screen.getByTestId('progress-chart')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toBeInTheDocument()
    expect(screen.getByLabelText('To')).toBeInTheDocument()
  })

  it('shows an empty progress message when the range has no data', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    mockUseSharedSteamProgress.mockReturnValue({ data: undefined, isLoading: false })
    render(<ProfileGamesClient token="tok-1" />)

    fireEvent.click(screen.getByRole('tab', { name: 'Progress' }))

    expect(screen.getByText('No progress data for this range.')).toBeInTheDocument()
  })
})
