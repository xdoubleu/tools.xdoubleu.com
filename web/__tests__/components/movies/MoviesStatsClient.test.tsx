import React from 'react'
import { render, screen, within } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'

const mockBarChart = jest.fn()

jest.mock('recharts', () => ({
  BarChart: (props: { data: unknown; children: React.ReactNode }) => {
    mockBarChart(props.data)
    return <div>{props.children}</div>
  },
  Bar: ({ dataKey, stackId }: { dataKey: string; stackId: string }) => (
    <span data-testid="bar">{`${dataKey}:${stackId}`}</span>
  ),
  XAxis: ({ dataKey }: { dataKey: string }) => <span data-testid="x-axis">{dataKey}</span>,
  YAxis: ({ allowDecimals }: { allowDecimals: boolean }) => (
    <span data-testid="y-axis">{String(allowDecimals)}</span>
  ),
  CartesianGrid: () => null,
  Tooltip: () => null,
  Legend: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>
}))

jest.mock('@/hooks/useMovies', () => ({ useMovieStats: jest.fn() }))

import MoviesStatsClient from '@/components/movies/MoviesStatsClient'
import { useMovieStats } from '@/hooks/useMovies'
import {
  GenreCountSchema,
  GetStatsResponseSchema,
  MonthWatchesSchema
} from '@/lib/gen/movies/v1/movies_pb'

function mockStats(value: { data?: unknown; error?: Error; isLoading?: boolean }) {
  // @ts-expect-error -- partial SWRResponse
  jest.mocked(useMovieStats).mockReturnValue({ isLoading: false, ...value })
}

const full = create(GetStatsResponseSchema, {
  months: [
    create(MonthWatchesSchema, { month: '2025-11', movies: 0, seasons: 2 }),
    create(MonthWatchesSchema, { month: '2026-01', movies: 3, seasons: 1 })
  ],
  genres: [
    create(GenreCountSchema, { genre: 'Drama', count: 4 }),
    create(GenreCountSchema, { genre: 'Action', count: 2 })
  ],
  ratings: [0, 1, 0, 0, 2],
  moviesWatched: 3,
  seriesWatched: 1,
  seasonsWatched: 5
})

beforeEach(() => jest.clearAllMocks())

describe('MoviesStatsClient', () => {
  it('shows loading and error states without data', () => {
    mockStats({ isLoading: true })
    const { rerender } = render(<MoviesStatsClient />)
    expect(screen.getByText(/Loading stats/)).toBeInTheDocument()

    mockStats({ error: new Error('nope') })
    rerender(<MoviesStatsClient />)
    expect(screen.getByRole('alert')).toBeInTheDocument()

    mockStats({})
    rerender(<MoviesStatsClient />)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText(/Loading/)).not.toBeInTheDocument()
  })

  it('keeps loaded stats while revalidating or after a failed refresh', () => {
    mockStats({ data: full, isLoading: true, error: new Error('nope') })
    render(<MoviesStatsClient />)
    expect(screen.getByText('Movies watched')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('asks to watch something first', () => {
    mockStats({ data: create(GetStatsResponseSchema, { seriesWatched: 0 }) })
    render(<MoviesStatsClient />)
    expect(screen.getByText(/Nothing watched yet/)).toBeInTheDocument()
  })

  it('shows totals, a stacked monthly chart, genres and ratings', () => {
    mockStats({ data: full })
    render(<MoviesStatsClient />)

    for (const [label, value] of [
      ['Movies watched', '3'],
      ['Series watched', '1'],
      ['Seasons watched', '5']
    ]) {
      expect(screen.getByText(label).parentElement).toHaveTextContent(`${label}${value}`)
    }

    expect(mockBarChart).toHaveBeenLastCalledWith([
      { month: 'Nov 25', Movies: 0, Seasons: 2 },
      { month: 'Jan 26', Movies: 3, Seasons: 1 }
    ])
    expect(screen.getByTestId('x-axis')).toHaveTextContent('month')
    expect(screen.getByTestId('y-axis')).toHaveTextContent('false')
    expect(screen.getAllByTestId('bar').map((b) => b.textContent)).toEqual([
      'Movies:watches',
      'Seasons:watches'
    ])

    const genres = within(screen.getAllByRole('list')[0]).getAllByRole('listitem')
    expect(genres.map((li) => li.textContent)).toEqual(['Drama4', 'Action2'])

    const ratings = within(screen.getAllByRole('list')[1]).getAllByRole('listitem')
    expect(ratings).toHaveLength(5)
    expect(within(ratings[0]).getByRole('img', { name: '5 out of 5 stars' })).toBeInTheDocument()
    expect(ratings.map((li) => li.textContent?.replace(/★/g, ''))).toEqual([
      '2',
      '0',
      '0',
      '1',
      '0'
    ])
  })

  it('says when there are no genres or ratings yet', () => {
    mockStats({
      data: create(GetStatsResponseSchema, { ratings: [0, 0, 0, 0, 0], moviesWatched: 1 })
    })
    render(<MoviesStatsClient />)
    expect(screen.getByText('No genres yet.')).toBeInTheDocument()
    expect(screen.getByText('No ratings yet.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it.each([
    ['seasons', { seasonsWatched: 1 }],
    ['a watched series', { seriesWatched: 1 }],
    ['a rating', { ratings: [0, 0, 1, 0, 0] }],
    ['a movie and a series', { moviesWatched: 1, seriesWatched: 1 }],
    ['a series and a season', { seriesWatched: 1, seasonsWatched: 1 }]
  ])('shows stats for %s alone', (_name, fields) => {
    mockStats({ data: create(GetStatsResponseSchema, fields) })
    render(<MoviesStatsClient />)
    expect(screen.queryByText(/Nothing watched yet/)).not.toBeInTheDocument()
    expect(screen.getByText('Movies watched')).toBeInTheDocument()
  })
})
