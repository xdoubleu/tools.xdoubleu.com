import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'

const mockRefresh = jest.fn()

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogSteam: jest.fn()
}))

jest.mock('@/lib/backlog/steamRefresh', () => ({
  useSteamRefresh: jest.fn()
}))

jest.mock('swr', () => ({ mutate: jest.fn() }))
import { mutate } from 'swr'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

import GamesLibrary from '@/components/backlog/GamesLibrary'
import { useBacklogSteam } from '@/hooks/useBacklog'
import { useSteamRefresh } from '@/lib/backlog/steamRefresh'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  SteamResponseSchema,
  GetSteamResponseSchema
} from '@/lib/gen/backlog/v1/games_pb'

const mockUseBacklogSteam = jest.mocked(useBacklogSteam)
const mockUseSteamRefresh = jest.mocked(useSteamRefresh)

function mockRefreshState(overrides: Partial<ReturnType<typeof useSteamRefresh>> = {}) {
  mockUseSteamRefresh.mockReturnValue({
    connected: true,
    isRefreshing: false,
    lastRefresh: null,
    refresh: mockRefresh,
    ...overrides
  })
}

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
        currentRate: '1/week'
      })
    }),
    error: undefined,
    isLoading: false
  })
}

describe('GamesLibrary', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockRefreshState()
  })

  it('renders the grouped backlog', () => {
    mockSteam()
    render(<GamesLibrary />)
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.getByText('Celeste')).toBeInTheDocument()
    expect(screen.getByText('In Progress (1)')).toBeInTheDocument()
    expect(screen.getByText('Not Started (1)')).toBeInTheDocument()
  })

  it('filters games by the search input', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games...'), {
      target: { value: 'hades' }
    })
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.queryByText('Celeste')).not.toBeInTheDocument()
  })

  it('shows a no-match message when the search excludes every game', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.change(screen.getByPlaceholderText('Search games...'), {
      target: { value: 'nonexistent' }
    })
    expect(screen.getByText('No games match your search.')).toBeInTheDocument()
  })

  it('triggers a refresh when the button is clicked', () => {
    mockSteam()
    render(<GamesLibrary />)
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(mockRefresh).toHaveBeenCalled()
  })

  it('disables the button and shows a refreshing label while syncing', () => {
    mockSteam()
    mockRefreshState({ isRefreshing: true })
    render(<GamesLibrary />)
    expect(screen.getByRole('button', { name: 'Refreshing...' })).toBeDisabled()
  })

  it('shows the last refresh time when not refreshing', () => {
    mockSteam()
    mockRefreshState({ lastRefresh: new Date('2026-01-02T03:04:05Z') })
    render(<GamesLibrary />)
    expect(screen.getByText(/^Last:/)).toBeInTheDocument()
  })

  it('re-fetches steam data when a sync completes', () => {
    mockSteam()
    render(<GamesLibrary />)
    const onSynced = mockUseSteamRefresh.mock.calls[0]![0]
    onSynced?.()
    expect(mutate).toHaveBeenCalledWith('/backlog/games')
  })

  it('links a game card to its detail page', () => {
    mockSteam()
    render(<GamesLibrary />)
    expect(screen.getByText('Hades').closest('a')).toHaveAttribute('href', '/backlog/games/10')
  })

  it('shows a loading state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
    render(<GamesLibrary />)
    expect(screen.getByText('Loading Steam library...')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({
      data: undefined,
      error: new Error('boom'),
      isLoading: false
    })
    render(<GamesLibrary />)
    expect(screen.getByText('Failed to load Steam data.')).toBeInTheDocument()
  })
})
