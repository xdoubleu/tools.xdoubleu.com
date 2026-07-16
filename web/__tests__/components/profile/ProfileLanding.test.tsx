import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { LibraryResponseSchema, UserBookSchema, BookSchema } from '@/lib/gen/books/v1/library_pb'
import { GetSharedLibraryResponseSchema } from '@/lib/gen/books/v1/public_pb'
import { SteamResponseSchema, GameSchema } from '@/lib/gen/games/v1/games_pb'
import { GetSharedSteamResponseSchema } from '@/lib/gen/games/v1/public_pb'

const mockUseSharedLibrary = jest.fn()
const mockUseSharedSteam = jest.fn()

jest.mock('@/hooks/useProfile', () => ({
  useSharedLibrary: () => mockUseSharedLibrary(),
  useSharedSteam: () => mockUseSharedSteam()
}))

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import ProfileLanding from '@/components/profile/ProfileLanding'

function makeLibrary() {
  return create(GetSharedLibraryResponseSchema, {
    library: create(LibraryResponseSchema, {
      reading: [
        create(UserBookSchema, {
          id: 'ub-1',
          status: 'currently-reading',
          tags: ['favourite'],
          book: create(BookSchema, { title: 'Fav Book', authors: ['Author A'] })
        })
      ],
      wishlist: [],
      finished: [],
      shelves: []
    }),
    lastSyncedAt: '2026-07-01T10:00:00Z'
  })
}

function makeSteam() {
  return create(GetSharedSteamResponseSchema, {
    steam: create(SteamResponseSchema, {
      notStarted: [create(GameSchema, { id: 1, name: 'Backlog Game' })],
      inProgress: [create(GameSchema, { id: 2, name: 'Fav Game', favourite: true })],
      completed: [],
      totalBacklog: 2,
      currentRate: '42.00'
    }),
    lastSyncedAt: '2026-07-01T10:00:00Z'
  })
}

describe('ProfileLanding', () => {
  beforeEach(() => jest.clearAllMocks())

  it('renders books and games sections with links to sub-pages', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })

    render(<ProfileLanding token="tok-1" />)

    expect(screen.getByRole('heading', { name: 'Books' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Games' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'View books' })).toHaveAttribute(
      'href',
      '/profile/tok-1/books'
    )
    expect(screen.getByRole('link', { name: 'View games' })).toHaveAttribute(
      'href',
      '/profile/tok-1/games'
    )
  })

  it('shows favourite books and games strips', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })

    render(<ProfileLanding token="tok-1" />)

    expect(screen.getByText('Favourite books')).toBeInTheDocument()
    expect(screen.getByText('Fav Book')).toBeInTheDocument()
    expect(screen.getByText('Favourite games')).toBeInTheDocument()
    expect(screen.getByText('Fav Game')).toBeInTheDocument()
  })

  it('has no refresh buttons', () => {
    mockUseSharedLibrary.mockReturnValue({ data: makeLibrary() })
    mockUseSharedSteam.mockReturnValue({ data: makeSteam() })

    render(<ProfileLanding token="tok-1" />)

    expect(screen.queryByRole('button', { name: /refresh/i })).not.toBeInTheDocument()
  })

  it('reports an invalid link when both sections fail', () => {
    mockUseSharedLibrary.mockReturnValue({ data: undefined, error: new Error('nope') })
    mockUseSharedSteam.mockReturnValue({ data: undefined, error: new Error('nope') })

    render(<ProfileLanding token="tok-1" />)

    expect(
      screen.getByText('This profile link is invalid or has been disabled.')
    ).toBeInTheDocument()
  })
})
