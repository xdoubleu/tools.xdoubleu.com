import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('@/hooks/useBacklog', () => ({
  useBacklogDistribution: jest.fn()
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

import SteamDistributionClient from '@/app/backlog/games/distribution/[bucket]/SteamDistributionClient'
import { useBacklogDistribution } from '@/hooks/useBacklog'
import { create } from '@bufbuild/protobuf'
import {
  GameSchema,
  GetSteamDistributionResponseSchema,
  SteamDistributionResponseSchema
} from '@/lib/gen/backlog/v1/games_pb'

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
  jest.clearAllMocks()
})

describe('SteamDistributionClient', () => {
  it('shows loading state when isLoading is true', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogDistribution).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<SteamDistributionClient bucket="8" />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('shows error state when error is present', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogDistribution).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('boom')
    })

    render(<SteamDistributionClient bucket="8" />)
    expect(screen.getByText('Failed to load distribution.')).toBeInTheDocument()
  })

  it('shows empty message when there are no games', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    jest.mocked(useBacklogDistribution).mockReturnValue({
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
    jest.mocked(useBacklogDistribution).mockReturnValue({
      data: mockResponse,
      isLoading: false,
      error: undefined
    })

    render(<SteamDistributionClient bucket="8" />)
    const link = screen.getByText('Hollow Knight').closest('a')
    expect(link).toHaveAttribute('href', '/backlog/games/42?bucket=8&label=80-89%25')
  })
})
