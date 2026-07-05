import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  SteamResponseSchema,
  GetSteamResponseSchema
} from '@/lib/gen/games/v1/games_pb'

jest.mock('@/hooks/useGames', () => ({
  useSteam: jest.fn()
}))

jest.mock('next/link', () => {
  return ({
    children,
    href,
    onClick
  }: {
    children: React.ReactNode
    href: string
    onClick?: () => void
  }) => (
    <a href={href} onClick={onClick}>
      {children}
    </a>
  )
})

import GamesSearch from '@/components/games/GamesSearch'
import { useSteam } from '@/hooks/useGames'

const mockUseBacklogSteam = jest.mocked(useSteam)

const GAMES = {
  hades: create(GameSchema, { id: 1, name: 'Hades' }),
  bastion: create(GameSchema, { id: 2, name: 'Bastion' }),
  transistor: create(GameSchema, { id: 3, name: 'Transistor' })
}

function mockSteam() {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogSteam.mockReturnValue({
    data: create(GetSteamResponseSchema, {
      steam: create(SteamResponseSchema, {
        inProgress: [GAMES.hades],
        notStarted: [GAMES.transistor],
        completed: [GAMES.bastion]
      })
    }),
    error: undefined,
    isLoading: false
  })
}

beforeEach(() => {
  jest.clearAllMocks()
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseBacklogSteam.mockReturnValue({ data: undefined, error: undefined, isLoading: true })
})

describe('GamesSearch', () => {
  it('renders the search input', () => {
    render(<GamesSearch />)
    expect(screen.getByPlaceholderText('Search games…')).toBeInTheDocument()
  })

  it('shows no dropdown when query is empty', () => {
    mockSteam()
    render(<GamesSearch />)
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('shows matching results when query matches game names', () => {
    mockSteam()
    render(<GamesSearch />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'ha' }
    })

    expect(screen.getByRole('list')).toBeInTheDocument()
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.queryByText('Bastion')).not.toBeInTheDocument()
    expect(screen.queryByText('Transistor')).not.toBeInTheDocument()
  })

  it('searches across all status groups', () => {
    mockSteam()
    render(<GamesSearch />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'a' }
    })

    // Hades (inProgress), Bastion (completed), Transistor (notStarted) all contain 'a'
    expect(screen.getByText('Hades')).toBeInTheDocument()
    expect(screen.getByText('Bastion')).toBeInTheDocument()
    expect(screen.getByText('Transistor')).toBeInTheDocument()
  })

  it('is case-insensitive', () => {
    mockSteam()
    render(<GamesSearch />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'HADES' }
    })

    expect(screen.getByText('Hades')).toBeInTheDocument()
  })

  it('shows no results when nothing matches', () => {
    mockSteam()
    render(<GamesSearch />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'zzznomatch' }
    })

    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('result links point to the correct game detail URL', () => {
    mockSteam()
    render(<GamesSearch />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'hades' }
    })

    const link = screen.getByRole('link', { name: 'Hades' })
    expect(link).toHaveAttribute('href', '/games/1')
  })

  it('clears the dropdown when a result is clicked', () => {
    mockSteam()
    render(<GamesSearch />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'hades' }
    })
    expect(screen.getByText('Hades')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('link', { name: 'Hades' }))
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('limits results to 5', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseBacklogSteam.mockReturnValue({
      data: create(GetSteamResponseSchema, {
        steam: create(SteamResponseSchema, {
          notStarted: Array.from({ length: 10 }, (_, i) =>
            create(GameSchema, { id: i + 1, name: `Game ${i + 1}` })
          )
        })
      }),
      error: undefined,
      isLoading: false
    })

    render(<GamesSearch />)

    fireEvent.change(screen.getByPlaceholderText('Search games…'), {
      target: { value: 'game' }
    })

    expect(screen.getAllByRole('listitem')).toHaveLength(5)
  })
})
