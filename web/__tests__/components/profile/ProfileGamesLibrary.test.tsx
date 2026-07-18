import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { SteamResponseSchema, GameSchema } from '@/lib/gen/games/v1/games_pb'
import { GetSharedSteamResponseSchema } from '@/lib/gen/games/v1/public_pb'

const mockUseSharedSteam = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useSharedSteam: () => mockUseSharedSteam()
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import ProfileGamesLibrary from '@/components/profile/ProfileGamesLibrary'

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

describe('ProfileGamesLibrary', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('groups games with a Favourites section first', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesLibrary token="tok-1" />)

    expect(screen.getByRole('heading', { name: 'Favourites (1)' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'In Progress (1)' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Not Started (1)' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Completed (1)' })).toBeInTheDocument()
  })

  it('links game cards to the public game pages', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesLibrary token="tok-1" />)

    const links = screen.getAllByRole('link')
    expect(links.some((l) => l.getAttribute('href') === '/profile/games/tok-1/2')).toBe(true)
    expect(links.every((l) => l.getAttribute('href')?.startsWith('/profile/games/tok-1/'))).toBe(
      true
    )
  })

  it('renders a loading state before data arrives', () => {
    mockUseSharedSteam.mockReturnValue({ data: undefined, isLoading: true })
    render(<ProfileGamesLibrary token="tok-1" />)

    expect(screen.getByText('Loading games…')).toBeInTheDocument()
  })

  it('shows an error state when steam data fails to load', () => {
    mockUseSharedSteam.mockReturnValue({ data: undefined, error: new Error('nope') })
    render(<ProfileGamesLibrary token="tok-1" />)

    expect(screen.getByText('Failed to load games.')).toBeInTheDocument()
  })

  it('filters games with the search input', () => {
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })
    render(<ProfileGamesLibrary token="tok-1" />)

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
})
