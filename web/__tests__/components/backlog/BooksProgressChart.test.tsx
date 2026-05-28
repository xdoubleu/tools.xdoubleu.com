import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import BooksProgressChart from '@/components/backlog/BooksProgressChart'
import {
  GetBooksProgressResponseSchema,
  BooksProgressResponseSchema
} from '@/lib/gen/backlog/v1/books_pb'

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
    const emptyData = create(GetBooksProgressResponseSchema, {
      progress: create(BooksProgressResponseSchema, { labels: [], values: [] })
    })
    render(<BooksProgressChart data={emptyData} />)
    expect(screen.getByText('No progress data available.')).toBeInTheDocument()
  })

  it('renders chart when progress data is provided', () => {
    const data = create(GetBooksProgressResponseSchema, {
      progress: create(BooksProgressResponseSchema, {
        labels: ['Jan', 'Feb', 'Mar'],
        values: ['2', '5', '3']
      })
    })
    render(<BooksProgressChart data={data} />)
    expect(screen.getByTestId('line-chart')).toBeInTheDocument()
  })
})
