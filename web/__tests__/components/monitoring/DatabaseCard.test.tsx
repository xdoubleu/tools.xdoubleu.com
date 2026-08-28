import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { GetDatabaseStatsResponseSchema } from '@/lib/gen/observability/v1/observability_pb'
import DatabaseCard from '@/components/monitoring/DatabaseCard'

describe('DatabaseCard', () => {
  it('shows loading state without data', () => {
    render(<DatabaseCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.getByText('No schema data.')).toBeInTheDocument()
  })

  it('renders schema sizes and total', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 1024n * 1024n,
      schemas: [{ name: 'global', sizeBytes: 1024n * 1024n, tableCount: 5n }],
      tableGrowth: []
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('1.0 MB total on disk')).toBeInTheDocument()
    expect(screen.getByText('global')).toBeInTheDocument()
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
