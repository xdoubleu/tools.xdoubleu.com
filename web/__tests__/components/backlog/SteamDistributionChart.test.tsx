import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import SteamDistributionChart from '@/components/backlog/SteamDistributionChart'

// Mock recharts so that the Bar receives the chart `data` (forwarded from
// BarChart) and renders one clickable element per entry. This lets the test
// exercise the real onClick -> onBucketClick wiring.
jest.mock('recharts', () => {
  const ReactLib = require('react')
  const Bar = ({ onClick, data }: { onClick?: (entry: unknown) => void; data?: unknown[] }) => (
    <>
      {(data ?? []).map((entry, i) => (
        <button key={i} data-testid={`bar-${i}`} onClick={() => onClick?.(entry)} />
      ))}
    </>
  )
  return {
    BarChart: ({ children, data }: { children: React.ReactNode; data: unknown[] }) => (
      <div data-testid="bar-chart">
        {ReactLib.Children.map(children, (child: React.ReactElement) =>
          child && child.type === Bar ? ReactLib.cloneElement(child, { data }) : child
        )}
      </div>
    ),
    Bar,
    XAxis: () => null,
    YAxis: () => null,
    CartesianGrid: () => null,
    Tooltip: () => null,
    Cell: () => null,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div data-testid="responsive-container">{children}</div>
    )
  }
})

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

  it('calls onBucketClick with the bucket index when a bar is clicked', () => {
    const distribution = [5, 10, 8, 3, 2, 1, 4, 6, 7, 9, 11]
    const onBucketClick = jest.fn()
    render(<SteamDistributionChart distribution={distribution} onBucketClick={onBucketClick} />)

    fireEvent.click(screen.getByTestId('bar-1'))
    expect(onBucketClick).toHaveBeenCalledWith(1)

    fireEvent.click(screen.getByTestId('bar-10'))
    expect(onBucketClick).toHaveBeenCalledWith(10)
  })
})
