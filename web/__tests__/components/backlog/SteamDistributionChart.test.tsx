import React from 'react'
import { render, screen } from '@testing-library/react'
import SteamDistributionChart from '@/components/backlog/SteamDistributionChart'
import type { GetSteamDistributionResponse } from '@/lib/gen/backlog/v1/games_pb'

jest.mock('recharts', () => ({
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  )
}))

describe('SteamDistributionChart', () => {
  it('renders empty state when no data', () => {
    render(<SteamDistributionChart data={undefined} />)
    expect(screen.getByText('No distribution data available.')).toBeInTheDocument()
  })

  it('renders empty state when data has no games', () => {
    const data: GetSteamDistributionResponse = {
      data: { label: '', games: [] }
    } as any
    render(<SteamDistributionChart data={data} />)
    expect(screen.getByText('No distribution data available.')).toBeInTheDocument()
  })

  it('renders chart when data is available', () => {
    const data: GetSteamDistributionResponse = {
      data: {
        label: 'test',
        games: [
          { id: 1, name: 'Game 1', isDelisted: false, completionRate: '25%', contribution: '', playtime: 0 },
          { id: 2, name: 'Game 2', isDelisted: false, completionRate: '50%', contribution: '', playtime: 0 },
          { id: 3, name: 'Game 3', isDelisted: false, completionRate: '75%', contribution: '', playtime: 0 }
        ]
      }
    } as any
    render(<SteamDistributionChart data={data} />)
    expect(screen.getByTestId('bar-chart')).toBeInTheDocument()
  })
})
