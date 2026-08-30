import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import {
  GetDatabaseSizeHistoryResponseSchema,
  DBSizeHistoryPointSchema
} from '@/lib/gen/observability/v1/observability_pb'
import DatabaseSizeHistoryCard, {
  toSeriesPoints,
  toSeriesMeta
} from '@/components/monitoring/DatabaseSizeHistoryCard'

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

describe('toSeriesPoints / toSeriesMeta', () => {
  const points = [
    create(DBSizeHistoryPointSchema, {
      day: '2026-01-01',
      schemaName: 'books',
      tableName: 'books',
      sizeBytes: 1000n
    }),
    create(DBSizeHistoryPointSchema, {
      day: '2026-01-02',
      schemaName: 'books',
      tableName: 'books',
      sizeBytes: 1500n
    }),
    create(DBSizeHistoryPointSchema, {
      day: '2026-01-01',
      schemaName: 'games',
      tableName: 'library_entries',
      sizeBytes: 500n
    })
  ]

  it('builds a seriesKey/value point per row', () => {
    expect(toSeriesPoints(points)).toEqual([
      { day: '2026-01-01', seriesKey: 'books.books', value: 1000 },
      { day: '2026-01-02', seriesKey: 'books.books', value: 1500 },
      { day: '2026-01-01', seriesKey: 'games.library_entries', value: 500 }
    ])
  })

  it('dedupes into one meta entry per (schema, table)', () => {
    expect(toSeriesMeta(points)).toEqual([
      { key: 'books.books', label: 'books.books' },
      { key: 'games.library_entries', label: 'games.library_entries' }
    ])
  })
})

describe('DatabaseSizeHistoryCard', () => {
  it('shows an empty state without data', () => {
    render(<DatabaseSizeHistoryCard data={undefined} />)
    expect(screen.getByText('No size history yet.')).toBeInTheDocument()
  })

  it('shows an empty state when points is empty', () => {
    const data = create(GetDatabaseSizeHistoryResponseSchema, { points: [] })
    render(<DatabaseSizeHistoryCard data={data} />)
    expect(screen.getByText('No size history yet.')).toBeInTheDocument()
  })

  it('renders the chart when points are present', () => {
    const data = create(GetDatabaseSizeHistoryResponseSchema, {
      points: [
        {
          day: '2026-01-01',
          schemaName: 'books',
          tableName: 'books',
          sizeBytes: 1000n
        }
      ]
    })
    render(<DatabaseSizeHistoryCard data={data} />)
    expect(screen.getByTestId('line-chart')).toBeInTheDocument()
    expect(screen.getByLabelText('books.books')).toBeInTheDocument()
  })
})
