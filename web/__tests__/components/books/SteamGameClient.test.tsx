import React from 'react'
import { act, render, screen, fireEvent } from '@testing-library/react'

const mockRefreshSteamGame = jest.fn()
const mockGlobalMutate = jest.fn()

jest.mock('swr', () => ({
  ...jest.requireActual('swr'),
  mutate: (...args: unknown[]) => mockGlobalMutate(...args)
}))

jest.mock('@/hooks/useGames', () => ({
  useSteamGame: jest.fn(),
  useRefreshSteamGame: jest.fn(() => mockRefreshSteamGame)
}))

let mockSearchParams = new URLSearchParams()
jest.mock('next/navigation', () => ({
  useSearchParams: () => mockSearchParams
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('next/image', () => {
  return function MockImage({
    src,
    alt,
    ...props
  }: {
    src: string
    alt: string
    [key: string]: unknown
  }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} {...props} />
  }
})

import SteamGameClient from '@/app/games/[id]/SteamGameClient'
import { useSteamGame, useRefreshSteamGame } from '@/hooks/useGames'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  AchievementSchema,
  GetSteamGameResponseSchema,
  SteamGameResponseSchema,
  RefreshSteamGameResponseSchema
} from '@/lib/gen/games/v1/games_pb'
import { swrKeys } from '@/lib/swrKeys'

const mockGame = create(GameSchema, {
  id: 1,
  name: 'The Witcher 3',
  playtime: 3600,
  completionRate: '85.00',
  isDelisted: false,
  lastSyncedAt: '2026-06-23T10:00:00Z'
})

const mockAchievements = [
  create(AchievementSchema, {
    name: 'ach1',
    displayName: 'Achievement 1',
    description: 'First achievement',
    achieved: true,
    globalPercent: 50.5,
    iconUrl: 'http://example.com/ach1.png'
  }),
  create(AchievementSchema, {
    name: 'ach2',
    displayName: 'Achievement 2',
    description: 'Second achievement',
    achieved: false,
    globalPercent: 25.3,
    iconUrl: 'http://example.com/ach2.png'
  })
]

const mockHiddenAchievement = create(AchievementSchema, {
  name: 'ach3',
  displayName: 'Secret Achievement',
  description: '',
  achieved: false,
  globalPercent: 1.2,
  iconUrl: 'http://example.com/ach3.png'
})

beforeEach(() => {
  jest.clearAllMocks()
  mockSearchParams = new URLSearchParams()
  mockRefreshSteamGame.mockResolvedValue(undefined)
  jest.mocked(useRefreshSteamGame).mockReturnValue(mockRefreshSteamGame)
})

const mockSteamGameResponse = (
  game: ReturnType<typeof create<typeof GameSchema>>,
  achievements: ReturnType<typeof create<typeof AchievementSchema>>[]
) =>
  create(GetSteamGameResponseSchema, {
    data: create(SteamGameResponseSchema, { game, achievements })
  })

describe('SteamGameClient', () => {
  it('shows loading state when isLoading is true', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Loading game…')).toBeInTheDocument()
  })

  it('shows error state when error is present', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('Failed to fetch')
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Failed to load game.')).toBeInTheDocument()
  })

  it('renders game name when game is loaded', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByRole('heading', { name: 'The Witcher 3' })).toBeInTheDocument()
  })

  it('renders achievements section with unlocked achievements by default', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Achievements/)).toBeInTheDocument()
    // Achievement 1 is achieved (completed) so it is hidden by default.
    expect(screen.queryByText('Achievement 1')).not.toBeInTheDocument()
    expect(screen.getByText('Achievement 2')).toBeInTheDocument()
  })

  it('gives each achievement card a solid card background', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const card = screen.getByText('Achievement 2').closest('div.bg-card')
    expect(card).toBeInTheDocument()
  })

  it('shows a Hidden badge for achievements without a description', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, [mockHiddenAchievement]),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Secret Achievement')).toBeInTheDocument()
    expect(screen.getByText('Hidden')).toBeInTheDocument()
  })

  it('does not show a Hidden badge for achievements with a description', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Achievement 2')).toBeInTheDocument()
    expect(screen.queryByText('Hidden')).not.toBeInTheDocument()
  })

  it('renders the achievement icon with a locked square aspect ratio', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByAltText('Achievement 2')).toHaveClass('h-12', 'w-12', 'object-cover')
  })

  it('orders achievements by descending global percent', () => {
    const achievements = [
      create(AchievementSchema, {
        name: 'low',
        displayName: 'Low percent',
        achieved: false,
        globalPercent: 10.5
      }),
      create(AchievementSchema, {
        name: 'high',
        displayName: 'High percent',
        achieved: false,
        globalPercent: 80.2
      }),
      create(AchievementSchema, {
        name: 'mid',
        displayName: 'Mid percent',
        achieved: false,
        globalPercent: 40.1
      })
    ]
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, achievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const headings = screen.getAllByRole('heading', { level: 3 }).map((h) => h.textContent)
    expect(headings).toEqual(['High percent', 'Mid percent', 'Low percent'])
  })

  it('toggles completed achievements visibility', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const toggle = screen.getByRole('button', { name: 'Show completed' })
    fireEvent.click(toggle)
    expect(screen.getByText('Achievement 1')).toBeInTheDocument()
    expect(screen.getByText('Achievement 2')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Hide completed' }))
    expect(screen.queryByText('Achievement 1')).not.toBeInTheDocument()
  })

  it('shows all-completed message when every achievement is unlocked', () => {
    const allDone = [
      create(AchievementSchema, {
        name: 'done',
        displayName: 'Done',
        achieved: true,
        globalPercent: 30
      })
    ]
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, allDone),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('All achievements completed.')).toBeInTheDocument()
  })

  it('shows no achievements message when list is empty', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('No achievements for this game.')).toBeInTheDocument()
  })

  it('calls useSteamGame with numeric id converted from string', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="456" />)
    expect(useSteamGame).toHaveBeenCalledWith(456, undefined)
  })

  it('renders games link', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const backlogLink = screen.getByText(/Games/).closest('a')
    expect(backlogLink).toHaveAttribute('href', '/games')
  })

  it('does not render a distribution breadcrumb without a bucket param', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.queryByText('80-89%')).not.toBeInTheDocument()
  })

  it('renders a distribution breadcrumb linking back to the bucket overview', () => {
    mockSearchParams = new URLSearchParams({ bucket: '8', label: '80-89%' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const bucketLink = screen.getByText('80-89%').closest('a')
    expect(bucketLink).toHaveAttribute('href', '/games/distribution/8')
  })

  it('falls back to a range label when only the bucket param is present', () => {
    mockSearchParams = new URLSearchParams({ bucket: '3' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const bucketLink = screen.getByText('3% range').closest('a')
    expect(bucketLink).toHaveAttribute('href', '/games/distribution/3')
  })

  it('displays playtime in hours', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/60\s+hrs played/)).toBeInTheDocument()
  })

  it('displays completion rate', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Completion: 85.00%')).toBeInTheDocument()
  })

  it('displays achieved count in achievements header', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Achievements \(1\/2\)/)).toBeInTheDocument()
  })

  it('renders achievement cards with correct statuses', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    // Locked is visible by default; Achieved appears once completed are shown.
    expect(screen.getByText('Locked')).toBeInTheDocument()
    expect(screen.queryByText('Achieved')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Show completed' }))
    expect(screen.getByText('Achieved')).toBeInTheDocument()
  })

  it('shows delisted badge when game is delisted', () => {
    const delistedGame = create(GameSchema, { ...mockGame, isDelisted: true })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(delistedGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Delisted')).toBeInTheDocument()
  })

  it('does not show delisted badge when game is not delisted', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.queryByText('Delisted')).not.toBeInTheDocument()
  })

  it('renders the Refresh and High poll buttons', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'High poll: off' })).toBeInTheDocument()
  })

  it('toggles high poll mode label', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    const toggle = screen.getByRole('button', { name: 'High poll: off' })
    fireEvent.click(toggle)
    expect(screen.getByRole('button', { name: 'High poll: on' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'High poll: on' }))
    expect(screen.getByRole('button', { name: 'High poll: off' })).toBeInTheDocument()
  })

  it('displays last synced timestamp', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Last synced:/)).toBeInTheDocument()
  })

  it('does not display last synced when lastSyncedAt is empty', () => {
    const gameNoSync = create(GameSchema, { ...mockGame, lastSyncedAt: '' })
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(gameNoSync, []),
      isLoading: false,
      error: undefined
    })

    render(<SteamGameClient id="123" />)
    expect(screen.queryByText(/Last synced:/)).not.toBeInTheDocument()
  })

  it('calls refreshSteamGame when Refresh button is clicked', async () => {
    const mutate = jest.fn().mockResolvedValue(undefined)
    const freshRefreshResponse = create(RefreshSteamGameResponseSchema, {
      data: create(SteamGameResponseSchema, { game: mockGame, achievements: [] })
    })
    mockRefreshSteamGame.mockResolvedValue(freshRefreshResponse)

    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useSteamGame).mockReturnValue({
      data: mockSteamGameResponse(mockGame, mockAchievements),
      isLoading: false,
      error: undefined,
      mutate
    })

    render(<SteamGameClient id="123" />)

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    })

    expect(mockRefreshSteamGame).toHaveBeenCalledWith(123)
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ data: freshRefreshResponse.data }),
      { revalidate: false }
    )
    expect(mockGlobalMutate).toHaveBeenCalledWith(swrKeys.games)
  })

  describe('live polling', () => {
    beforeEach(() => {
      jest.useFakeTimers()
    })

    afterEach(() => {
      jest.useRealTimers()
    })

    it('does not poll by default (high poll mode off)', async () => {
      // @ts-expect-error -- mock returns partial SWRResponse for test purposes
      jest.mocked(useSteamGame).mockReturnValue({
        data: mockSteamGameResponse(mockGame, []),
        isLoading: false,
        error: undefined,
        mutate: jest.fn()
      })

      render(<SteamGameClient id="123" />)

      await act(async () => {
        jest.advanceTimersByTime(60_000)
      })

      expect(mockRefreshSteamGame).not.toHaveBeenCalled()
    })

    it('calls refreshSteamGame after 60s when high poll mode is enabled', async () => {
      const mutate = jest.fn().mockResolvedValue(undefined)
      const freshRefreshResponse = create(RefreshSteamGameResponseSchema, {
        data: create(SteamGameResponseSchema, { game: mockGame, achievements: [] })
      })
      mockRefreshSteamGame.mockResolvedValue(freshRefreshResponse)

      // @ts-expect-error -- mock returns partial SWRResponse for test purposes
      jest.mocked(useSteamGame).mockReturnValue({
        data: mockSteamGameResponse(mockGame, mockAchievements),
        isLoading: false,
        error: undefined,
        mutate
      })

      render(<SteamGameClient id="123" />)

      // Enable high poll mode.
      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'High poll: off' }))
      })

      // No call before interval fires.
      expect(mockRefreshSteamGame).not.toHaveBeenCalled()

      await act(async () => {
        jest.advanceTimersByTime(60_000)
      })

      expect(mockRefreshSteamGame).toHaveBeenCalledWith(123)
      expect(mutate).toHaveBeenCalledWith(
        expect.objectContaining({ data: freshRefreshResponse.data }),
        { revalidate: false }
      )
    })

    it('does not call refreshSteamGame when the tab is hidden', async () => {
      // @ts-expect-error -- mock returns partial SWRResponse for test purposes
      jest.mocked(useSteamGame).mockReturnValue({
        data: mockSteamGameResponse(mockGame, []),
        isLoading: false,
        error: undefined,
        mutate: jest.fn()
      })

      Object.defineProperty(document, 'hidden', { value: true, writable: true })

      render(<SteamGameClient id="123" />)

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'High poll: off' }))
      })

      await act(async () => {
        jest.advanceTimersByTime(60_000)
      })

      expect(mockRefreshSteamGame).not.toHaveBeenCalled()

      Object.defineProperty(document, 'hidden', { value: false, writable: true })
    })

    it('clears the interval on unmount', async () => {
      const mutate = jest.fn()
      // @ts-expect-error -- mock returns partial SWRResponse for test purposes
      jest.mocked(useSteamGame).mockReturnValue({
        data: mockSteamGameResponse(mockGame, []),
        isLoading: false,
        error: undefined,
        mutate
      })

      const { unmount } = render(<SteamGameClient id="123" />)

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'High poll: off' }))
      })

      unmount()

      await act(async () => {
        jest.advanceTimersByTime(120_000)
      })

      expect(mockRefreshSteamGame).not.toHaveBeenCalled()
    })

    it('stops polling when high poll mode is toggled off', async () => {
      const mutate = jest.fn().mockResolvedValue(undefined)
      const freshRefreshResponse = create(RefreshSteamGameResponseSchema, {
        data: create(SteamGameResponseSchema, { game: mockGame, achievements: [] })
      })
      mockRefreshSteamGame.mockResolvedValue(freshRefreshResponse)

      // @ts-expect-error -- mock returns partial SWRResponse for test purposes
      jest.mocked(useSteamGame).mockReturnValue({
        data: mockSteamGameResponse(mockGame, []),
        isLoading: false,
        error: undefined,
        mutate
      })

      render(<SteamGameClient id="123" />)

      // Enable then disable.
      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'High poll: off' }))
      })
      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'High poll: on' }))
      })

      await act(async () => {
        jest.advanceTimersByTime(60_000)
      })

      expect(mockRefreshSteamGame).not.toHaveBeenCalled()
    })

    it('keeps showing prior data when refreshSteamGame rejects', async () => {
      const mutate = jest.fn()
      mockRefreshSteamGame.mockRejectedValue(new Error('network error'))

      // @ts-expect-error -- mock returns partial SWRResponse for test purposes
      jest.mocked(useSteamGame).mockReturnValue({
        data: mockSteamGameResponse(mockGame, mockAchievements),
        isLoading: false,
        error: undefined,
        mutate
      })

      render(<SteamGameClient id="123" />)

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: 'High poll: off' }))
      })

      await act(async () => {
        jest.advanceTimersByTime(60_000)
      })

      // mutate must not be called on failure; prior data remains.
      expect(mutate).not.toHaveBeenCalled()
      expect(screen.getByText('Achievement 2')).toBeInTheDocument()
    })
  })
})
