import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GameSchema, AchievementSchema, SteamGameResponseSchema } from '@/lib/gen/games/v1/games_pb'
import { GetSharedSteamGameResponseSchema } from '@/lib/gen/games/v1/public_pb'

const mockUseSharedSteamGame = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useSharedSteamGame: () => mockUseSharedSteamGame()
}))

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

import ProfileGameClient from '@/components/profile/ProfileGameClient'

function makeGame() {
  return create(GetSharedSteamGameResponseSchema, {
    data: create(SteamGameResponseSchema, {
      game: create(GameSchema, {
        id: 7,
        name: 'Public Game',
        favourite: true,
        completionRate: '75.00',
        playtime: 120,
        lastSyncedAt: '2026-07-01T10:00:00Z'
      }),
      achievements: [
        create(AchievementSchema, {
          name: 'ach-1',
          displayName: 'First Blood',
          description: 'Do the thing',
          achieved: true,
          globalPercent: 80.5
        }),
        create(AchievementSchema, {
          name: 'ach-2',
          displayName: 'Locked One',
          description: 'Not yet',
          achieved: false,
          globalPercent: 10.1
        })
      ]
    })
  })
}

describe('ProfileGameClient', () => {
  beforeEach(() => jest.clearAllMocks())

  it('renders the game with favourite marker and last synced state', () => {
    mockUseSharedSteamGame.mockReturnValue({ data: makeGame() })
    render(<ProfileGameClient token="tok-1" id="7" />)

    expect(screen.getByRole('heading', { name: /Public Game/ })).toBeInTheDocument()
    expect(screen.getByLabelText('Favourite')).toBeInTheDocument()
    expect(screen.getByText(/Last synced:/)).toBeInTheDocument()
    expect(screen.getByText('Completion: 75.00%')).toBeInTheDocument()
  })

  it('shows unachieved achievements by default and can reveal completed ones', () => {
    mockUseSharedSteamGame.mockReturnValue({ data: makeGame() })
    render(<ProfileGameClient token="tok-1" id="7" />)

    expect(screen.getByText('Locked One')).toBeInTheDocument()
    expect(screen.queryByText('First Blood')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Show completed' }))
    expect(screen.getByText('First Blood')).toBeInTheDocument()
  })

  it('is read-only: no refresh or high-poll controls', () => {
    mockUseSharedSteamGame.mockReturnValue({ data: makeGame() })
    render(<ProfileGameClient token="tok-1" id="7" />)

    expect(screen.queryByRole('button', { name: /refresh/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /high poll/i })).not.toBeInTheDocument()
  })

  it('links the breadcrumb back to the games profile', () => {
    mockUseSharedSteamGame.mockReturnValue({ data: makeGame() })
    render(<ProfileGameClient token="tok-1" id="7" />)

    const links = screen.getAllByRole('link')
    expect(links.some((l) => l.getAttribute('href') === '/profile/games/tok-1')).toBe(true)
  })

  it('shows an error state when the game fails to load', () => {
    mockUseSharedSteamGame.mockReturnValue({ data: undefined, error: new Error('nope') })
    render(<ProfileGameClient token="tok-1" id="7" />)

    expect(screen.getByText('Failed to load game.')).toBeInTheDocument()
  })
})
