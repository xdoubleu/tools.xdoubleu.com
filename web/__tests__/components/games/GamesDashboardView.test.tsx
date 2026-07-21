import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { GameSchema, RecentGameSchema, SteamResponseSchema } from '@/lib/gen/games/v1/games_pb'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'

jest.mock('@/components/games/SteamDistributionChart', () => {
  return function MockSteamDistributionChart({
    onBucketClick
  }: {
    onBucketClick?: (bucket: number) => void
  }) {
    return (
      <button data-testid="distribution-chart" onClick={() => onBucketClick?.(3)}>
        chart
      </button>
    )
  }
})
jest.mock('@/components/games/SteamProgressChart', () => () => <div data-testid="progress-chart" />)

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

jest.mock('next/image', () => {
  return function MockImage({ src, alt, ...props }: { src: string; alt: string }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} {...props} />
  }
})

import GamesDashboardView from '@/components/games/GamesDashboardView'

function makeSteam() {
  return create(SteamResponseSchema, {
    notStarted: [create(GameSchema, { id: 1, name: 'Backlog' })],
    inProgress: [create(GameSchema, { id: 2, name: 'Fav', favourite: true })],
    completed: [create(GameSchema, { id: 3, name: 'Done', favourite: true })],
    totalBacklog: 7,
    currentRate: '42.00',
    distribution: [1, 2, 3]
  })
}

function makeChart(
  view: 'progress' | 'distribution' = 'distribution'
): DashboardChartState<'progress' | 'distribution'> {
  return {
    view,
    setView: jest.fn(),
    start: '2025-01-01',
    setStart: jest.fn(),
    end: '2026-01-01',
    setEnd: jest.fn()
  }
}

const recent = [
  create(RecentGameSchema, {
    id: 2,
    name: 'Fav',
    completionRate: '50.00',
    playtime: 120,
    lastPlayedAt: '2026-06-01'
  })
]

describe('GamesDashboardView', () => {
  it('renders all five stat cards including a favourites count', () => {
    render(
      <GamesDashboardView
        steam={makeSteam()}
        recentGames={recent}
        gameHref={(g) => `/games/${g.id}`}
        chart={makeChart()}
        progressChartData={[]}
        favouritesHref="/games/library"
        actions={null}
      />
    )
    expect(screen.getByText('Total backlog')).toBeInTheDocument()
    expect(screen.getByText('42.00%')).toBeInTheDocument()
    expect(screen.getByText('In progress')).toBeInTheDocument()
    expect(screen.getByText('Completed')).toBeInTheDocument()
    const favourites = screen.getByText('Favourites').closest('a')
    expect(favourites).toHaveAttribute('href', '/games/library')
    // Two favourite games across inProgress + completed.
    expect(favourites).toHaveTextContent('2')
  })

  it('links recent cards to the supplied href', () => {
    render(
      <GamesDashboardView
        steam={makeSteam()}
        recentGames={recent}
        gameHref={(g) => `/profile/games/tok/${g.id}`}
        chart={makeChart()}
        progressChartData={[]}
        actions={null}
      />
    )
    expect(screen.getByText('Fav').closest('a')).toHaveAttribute('href', '/profile/games/tok/2')
  })

  it('renders the actions slot', () => {
    render(
      <GamesDashboardView
        steam={makeSteam()}
        recentGames={recent}
        gameHref={(g) => `/games/${g.id}`}
        chart={makeChart()}
        progressChartData={[]}
        actions={<button>Refresh</button>}
      />
    )
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeInTheDocument()
  })

  it('fires onBucketClick when a distribution bar is clicked', () => {
    const onBucketClick = jest.fn()
    render(
      <GamesDashboardView
        steam={makeSteam()}
        recentGames={recent}
        gameHref={(g) => `/games/${g.id}`}
        chart={makeChart()}
        progressChartData={[]}
        onBucketClick={onBucketClick}
        actions={null}
      />
    )
    fireEvent.click(screen.getByTestId('distribution-chart'))
    expect(onBucketClick).toHaveBeenCalledWith(3)
  })

  it('is inert on bucket click when no handler is supplied', () => {
    render(
      <GamesDashboardView
        steam={makeSteam()}
        recentGames={recent}
        gameHref={(g) => `/games/${g.id}`}
        chart={makeChart()}
        progressChartData={[]}
        actions={null}
      />
    )
    // No onBucketClick: clicking must not throw.
    expect(() => fireEvent.click(screen.getByTestId('distribution-chart'))).not.toThrow()
  })

  it('renders the progress chart in the progress view', () => {
    render(
      <GamesDashboardView
        steam={makeSteam()}
        recentGames={recent}
        gameHref={(g) => `/games/${g.id}`}
        chart={makeChart('progress')}
        progressChartData={[{ label: 'Jan', value: 10 }]}
        actions={null}
      />
    )
    expect(screen.getByTestId('progress-chart')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toBeInTheDocument()
  })
})
