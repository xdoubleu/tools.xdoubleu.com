import React from 'react'
import { render, screen } from '@testing-library/react'
import BooksProgressChart from '@/components/backlog/BooksProgressChart'

jest.mock('recharts', () => ({
  LineChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="line-chart">{children}</div>
  ),
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>
}))

describe('BooksProgressChart', () => {
  it('shows empty state when data is undefined', () => {
    render(<BooksProgressChart data={undefined} />)
    expect(screen.getByText('No progress data available.')).toBeInTheDocument()
  })

  it('shows empty state when data has no progress labels', () => {
    render(<BooksProgressChart data={{ progress: { labels: [], values: [] } } as never} />)
    expect(screen.getByText('No progress data available.')).toBeInTheDocument()
  })

  it('renders chart when progress data is provided', () => {
    const data = {
      progress: {
        labels: ['Jan', 'Feb', 'Mar'],
        values: ['2', '5', '3']
      }
    }
    render(<BooksProgressChart data={data as never} />)
    expect(screen.getByTestId('line-chart')).toBeInTheDocument()
  })
})
