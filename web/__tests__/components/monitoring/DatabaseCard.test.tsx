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
  })

  it('renders schema sizes and total', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 1024n * 1024n,
      schemas: [{ name: 'global', sizeBytes: 1024n * 1024n, tableCount: 5n }]
    })
    render(<DatabaseCard data={data} />)
    expect(screen.getByText('1.0 MB total on disk')).toBeInTheDocument()
    expect(screen.getAllByText('global').length).toBeGreaterThan(0)
  })
})
