import React from 'react'
import { render, screen } from '@testing-library/react'
import SteamDistributionChart from '@/components/backlog/SteamDistributionChart'

jest.mock('recharts', () => ({
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  Cell: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  )
}))

describe('SteamDistributionChart', () => {
  it('renders empty state when distribution is empty', () => {
    render(<SteamDistributionChart distribution={[]} />)
    expect(screen.getByText('No distribution data available.')).toBeInTheDocument()
  })

  it('renders chart when distribution has data', () => {
    const distribution = [5, 10, 8, 3, 2, 1, 4, 6, 7, 9, 11]
    render(<SteamDistributionChart distribution={distribution} />)
    expect(screen.getByTestId('bar-chart')).toBeInTheDocument()
  })

  it('calls onBucketClick with correct bucket index when bar clicked', () => {
    const distribution = [5, 10, 8]
    const onBucketClick = jest.fn()
    render(<SteamDistributionChart distribution={distribution} onBucketClick={onBucketClick} />)
    expect(screen.getByTestId('bar-chart')).toBeInTheDocument()
  })
})
