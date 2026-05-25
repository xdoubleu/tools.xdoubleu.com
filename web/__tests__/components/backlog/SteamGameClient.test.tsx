import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogSteamGame: jest.fn()
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

import SteamGameClient from '@/app/backlog/steam/[id]/SteamGameClient'
import { useBacklogSteamGame } from '@/hooks/useBacklog'
import type { Game, Achievement } from '@/lib/gen/backlog/v1/games_pb'

const mockGame = {
  id: 'game-1',
  name: 'The Witcher 3',
  playtime: 3600,
  completionRate: '85%',
  isDelisted: false
} as unknown as Game

const mockAchievements = [
  {
    name: 'ach1',
    displayName: 'Achievement 1',
    description: 'First achievement',
    achieved: true,
    globalPercent: 50.5,
    iconUrl: 'http://example.com/ach1.png'
  },
  {
    name: 'ach2',
    displayName: 'Achievement 2',
    description: 'Second achievement',
    achieved: false,
    globalPercent: 25.3,
    iconUrl: 'http://example.com/ach2.png'
  }
] as unknown as Achievement[]

beforeEach(() => {
  jest.clearAllMocks()
})

describe('SteamGameClient', () => {
  it('shows loading state when isLoading is true', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: null,
      isLoading: true,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Loading game...')).toBeInTheDocument()
  })

  it('shows error state when error is present', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: null,
      isLoading: false,
      error: new Error('Failed to fetch')
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Failed to load game.')).toBeInTheDocument()
  })

  it('renders game name when game is loaded', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: [] } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('The Witcher 3')).toBeInTheDocument()
  })

  it('renders achievements section when achievements exist', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: mockAchievements } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Achievements/)).toBeInTheDocument()
    expect(screen.getByText('Achievement 1')).toBeInTheDocument()
    expect(screen.getByText('Achievement 2')).toBeInTheDocument()
  })

  it('shows no achievements message when list is empty', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: [] } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('No achievements for this game.')).toBeInTheDocument()
  })

  it('calls useBacklogSteamGame with numeric id converted from string', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: null,
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="456" />)
    expect(useBacklogSteamGame).toHaveBeenCalledWith(456)
  })

  it('renders backlog link', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: null,
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    const backlogLink = screen.getByText(/Backlog/).closest('a')
    expect(backlogLink).toHaveAttribute('href', '/backlog')
  })

  it('displays playtime in hours', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: [] } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/60\s+hrs played/)).toBeInTheDocument()
  })

  it('displays completion rate', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: [] } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Completion: 85%')).toBeInTheDocument()
  })

  it('displays achieved count in achievements header', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: mockAchievements } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText(/Achievements \(1\/2\)/)).toBeInTheDocument()
  })

  it('renders achievement cards with correct statuses', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: mockAchievements } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Achieved')).toBeInTheDocument()
    expect(screen.getByText('Locked')).toBeInTheDocument()
  })

  it('shows delisted badge when game is delisted', () => {
    const delistedGame = { ...mockGame, isDelisted: true }
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: delistedGame, achievements: [] } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.getByText('Delisted')).toBeInTheDocument()
  })

  it('does not show delisted badge when game is not delisted', () => {
    ;(useBacklogSteamGame as jest.Mock).mockReturnValue({
      data: { data: { game: mockGame, achievements: [] } },
      isLoading: false,
      error: null
    })

    render(<SteamGameClient id="123" />)
    expect(screen.queryByText('Delisted')).not.toBeInTheDocument()
  })
})
