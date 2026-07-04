import React from 'react'
import { render, screen } from '@testing-library/react'
import BooksProgressChart from '@/components/books/BooksProgressChart'
import type { ProgressPoint } from '@/components/books/BooksProgressChart'

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
  it('shows empty state when data is an empty array', () => {
    render(<BooksProgressChart data={[]} />)
    expect(screen.getByText('No progress data available.')).toBeInTheDocument()
  })

  it('renders chart when data points are provided', () => {
    const data: ProgressPoint[] = [
      { label: '2026-01-01', value: 1 },
      { label: '2026-02-01', value: 3 },
      { label: '2026-03-01', value: 5 }
    ]
    render(<BooksProgressChart data={data} />)
    expect(screen.getByTestId('line-chart')).toBeInTheDocument()
  })
})
