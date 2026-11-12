import React from 'react'
import { render, screen, within } from '@testing-library/react'

const mockSetGameFavourite = vi.fn().mockResolvedValue({})

vi.mock('@/hooks/useGames', () => ({
  useSteamDistribution: vi.fn(),
  useSetGameFavourite: () => mockSetGameFavourite
}))

import SteamDistributionClient from '@/app/games/distribution/[bucket]/SteamDistributionClient'
import { useSteamDistribution } from '@/hooks/useGames'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  GetSteamDistributionResponseSchema,
  SteamDistributionResponseSchema
} from '@/lib/gen/games/v1/games_pb'

const mockGame = create(GameSchema, {
  id: 42,
  name: 'Hollow Knight',
  playtime: 1200,
  completionRate: '85.00'
})

const mockResponse = create(GetSteamDistributionResponseSchema, {
  data: create(SteamDistributionResponseSchema, { label: '80-89%', games: [mockGame] })
})

beforeEach(() => {
  vi.clearAllMocks()
})

describe('SteamDistributionClient', () => {
  it('shows loading state when isLoading is true', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useSteamDistribution).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<SteamDistributionClient bucket="8" />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows error state when error is present', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useSteamDistribution).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('boom')
    })

    render(<SteamDistributionClient bucket="8" />)
    expect(screen.getByText('Failed to load distribution.')).toBeInTheDocument()
  })

  it('trails the breadcrumb through the games library to the bucket', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useSteamDistribution).mockReturnValue({
      data: mockResponse,
      isLoading: false,
      error: undefined
    })
    render(<SteamDistributionClient bucket="8" />)
    const crumbs = within(screen.getByRole('navigation', { name: 'Breadcrumb' }))
    expect(crumbs.getByRole('link', { name: 'Games' })).toHaveAttribute('href', '/games/library')
    expect(crumbs.getByText('80-89%')).toBeInTheDocument()
  })

  it('shows empty message when there are no games', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useSteamDistribution).mockReturnValue({
      data: create(GetSteamDistributionResponseSchema, {
        data: create(SteamDistributionResponseSchema, { label: '80-89%', games: [] })
      }),
      isLoading: false,
      error: undefined
    })

    render(<SteamDistributionClient bucket="8" />)
    expect(screen.getByText('No games in this range.')).toBeInTheDocument()
  })

  it('links each game back to its bucket so the breadcrumb can return here', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    vi.mocked(useSteamDistribution).mockReturnValue({
      data: mockResponse,
      isLoading: false,
      error: undefined
    })

    render(<SteamDistributionClient bucket="8" />)
    const link = screen.getByText('Hollow Knight').closest('a')
    expect(link).toHaveAttribute('href', '/games/42?bucket=8&label=80-89%25')
  })
})
