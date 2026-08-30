import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import {
  GetTransactionLatencyHistoryResponseSchema,
  TransactionLatencyPointSchema
} from '@/lib/gen/observability/v1/observability_pb'
import TransactionLatencyHistoryCard, {
  toSeriesPoints,
  toSeriesMeta
} from '@/components/monitoring/TransactionLatencyHistoryCard'

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
    create(TransactionLatencyPointSchema, {
      day: '2026-01-01',
      project: 'tools-api',
      transaction: 'GET /a',
      p95DurationMs: 100
    }),
    create(TransactionLatencyPointSchema, {
      day: '2026-01-02',
      project: 'tools-api',
      transaction: 'GET /a',
      p95DurationMs: 150
    }),
    create(TransactionLatencyPointSchema, {
      day: '2026-01-01',
      project: 'tools-web',
      transaction: 'GET /b',
      p95DurationMs: 50
    })
  ]

  it('builds a seriesKey/value point per row', () => {
    expect(toSeriesPoints(points)).toEqual([
      { day: '2026-01-01', seriesKey: 'tools-api/GET /a', value: 100 },
      { day: '2026-01-02', seriesKey: 'tools-api/GET /a', value: 150 },
      { day: '2026-01-01', seriesKey: 'tools-web/GET /b', value: 50 }
    ])
  })

  it('dedupes into one meta entry per (project, transaction)', () => {
    expect(toSeriesMeta(points)).toEqual([
      { key: 'tools-api/GET /a', label: 'tools-api · GET /a' },
      { key: 'tools-web/GET /b', label: 'tools-web · GET /b' }
    ])
  })
})

describe('TransactionLatencyHistoryCard', () => {
  it('shows an empty state without data', () => {
    render(<TransactionLatencyHistoryCard data={undefined} />)
    expect(screen.getByText('No latency history yet.')).toBeInTheDocument()
  })

  it('shows an empty state when points is empty', () => {
    const data = create(GetTransactionLatencyHistoryResponseSchema, { points: [] })
    render(<TransactionLatencyHistoryCard data={data} />)
    expect(screen.getByText('No latency history yet.')).toBeInTheDocument()
  })

  it('renders the chart when points are present', () => {
    const data = create(GetTransactionLatencyHistoryResponseSchema, {
      points: [
        {
          day: '2026-01-01',
          project: 'tools-api',
          transaction: 'GET /a',
          p95DurationMs: 100,
          requestCount: 10n
        }
      ]
    })
    render(<TransactionLatencyHistoryCard data={data} />)
    expect(screen.getByTestId('line-chart')).toBeInTheDocument()
    expect(screen.getByLabelText('tools-api · GET /a')).toBeInTheDocument()
  })
})
