import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetDatabaseStatsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import DatabaseCard from '@/components/monitoring/DatabaseCard'

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

describe('DatabaseCard', () => {
  it('shows loading state without data', () => {
    render(<DatabaseCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.getByText('No schema data.')).toBeInTheDocument()
    expect(screen.getByText('No snapshot history.')).toBeInTheDocument()
  })

  it('renders the size-over-time chart when history is present', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 2048n,
      schemas: [],
      tableGrowth: [],
      history: [
        { sampledAt: '2026-08-28T00:00:00Z', totalSizeBytes: 1024n },
        { sampledAt: '2026-08-29T00:00:00Z', totalSizeBytes: 2048n }
      ]
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('Size over time')).toBeInTheDocument()
    expect(screen.queryByText('No snapshot history.')).not.toBeInTheDocument()
  })

  it('renders schema sizes and total', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 1024n * 1024n,
      schemas: [{ name: 'global', sizeBytes: 1024n * 1024n, tableCount: 5n }],
      tableGrowth: []
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('1.0 MB total on disk')).toBeInTheDocument()
    expect(screen.getAllByText('global').length).toBeGreaterThan(0)
    expect(screen.queryByText('Growing tables')).not.toBeInTheDocument()
  })

  it('renders growing tables with size, delta and percentage', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 1024n,
      schemas: [],
      tableGrowth: [
        {
          schemaName: 'global',
          tableName: 'usage_daily',
          currentSizeBytes: 1500n,
          deltaBytes: 500n,
          pctChange: 0.5
        }
      ]
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('Growing tables')).toBeInTheDocument()
    expect(screen.getByText('global.usage_daily')).toBeInTheDocument()
    expect(screen.getByText('+50%')).toBeInTheDocument()
  })

  it('renders a shrinking table with a negative delta', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 1024n,
      schemas: [],
      tableGrowth: [
        {
          schemaName: 'global',
          tableName: 'log_entries',
          currentSizeBytes: 500n,
          deltaBytes: -500n,
          pctChange: -0.5
        }
      ]
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('-50%')).toBeInTheDocument()
  })
})
