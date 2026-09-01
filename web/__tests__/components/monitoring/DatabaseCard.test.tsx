import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import {
  GetDatabaseStatsResponseSchema,
  GetDatabaseSizeHistoryResponseSchema,
  DBSizeHistoryPointSchema
} from '@/lib/gen/observability/v1/observability_pb'
import DatabaseCard, { toSeriesPoints, toSeriesMeta } from '@/components/monitoring/DatabaseCard'

// recharts needs a non-zero layout size that jsdom does not provide.
jest.mock('recharts', () => {
  const Original = jest.requireActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({
      children
    }: {
      children: React.ReactElement<{ width?: number; height?: number }>
    }) => <div>{React.cloneElement(children, { width: 400, height: 300 })}</div>
  }
})

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

describe('DatabaseCard', () => {
  it('shows loading state without data', () => {
    render(<DatabaseCard data={undefined} history={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.getByText('No schema data.')).toBeInTheDocument()
    expect(screen.getByText('No snapshot history.')).toBeInTheDocument()
    expect(screen.getByText('No size history yet.')).toBeInTheDocument()
  })

  it('renders the size-over-time chart when history is present', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 2048n,
      schemas: [],
      history: [
        { sampledAt: '2026-08-28T00:00:00Z', totalSizeBytes: 1024n },
        { sampledAt: '2026-08-29T00:00:00Z', totalSizeBytes: 2048n }
      ]
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('Total size over time')).toBeInTheDocument()
    expect(screen.queryByText('No snapshot history.')).not.toBeInTheDocument()
  })

  it('renders schema sizes and total, with no growing-tables section', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 1024n * 1024n,
      schemas: [{ name: 'global', sizeBytes: 1024n * 1024n, tableCount: 5n }]
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('1.0 MB total on disk')).toBeInTheDocument()
    expect(screen.getAllByText('global').length).toBeGreaterThan(0)
    expect(screen.queryByText('Growing tables')).not.toBeInTheDocument()
  })

  it('shows an empty state for schema/table history without data', () => {
    const data = create(GetDatabaseStatsResponseSchema, { totalSizeBytes: 1024n, schemas: [] })
    render(<DatabaseCard data={data} history={undefined} />)
    expect(screen.getByText('No size history yet.')).toBeInTheDocument()
  })

  it('shows an empty state for schema/table history when points is empty', () => {
    const data = create(GetDatabaseStatsResponseSchema, { totalSizeBytes: 1024n, schemas: [] })
    const history = create(GetDatabaseSizeHistoryResponseSchema, { points: [] })
    render(<DatabaseCard data={data} history={history} />)
    expect(screen.getByText('No size history yet.')).toBeInTheDocument()
  })

  it('renders the filterable schema/table history chart when points are present', () => {
    const data = create(GetDatabaseStatsResponseSchema, { totalSizeBytes: 1024n, schemas: [] })
    const history = create(GetDatabaseSizeHistoryResponseSchema, {
      points: [
        {
          day: '2026-01-01',
          schemaName: 'books',
          tableName: 'books',
          sizeBytes: 1000n
        }
      ]
    })
    render(<DatabaseCard data={data} history={history} />)
    expect(screen.getByText('Schema & table history')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Filter schemas or tables…')).toBeInTheDocument()
    expect(screen.getByLabelText('books.books')).toBeInTheDocument()
  })
})
