import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import {
  GetJobStatsResponseSchema,
  GetStorageStatsResponseSchema,
  GetDatabaseStatsResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import JobsCard from '@/components/monitoring/JobsCard'
import StorageCard from '@/components/monitoring/StorageCard'
import DatabaseCard from '@/components/monitoring/DatabaseCard'

// recharts needs a non-zero layout size that jsdom does not provide.
jest.mock('recharts', () => {
  const Original = jest.requireActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 400, height: 300 }}>{children}</div>
    )
  }
})

describe('JobsCard', () => {
  it('renders job stats and recent failures', () => {
    const data = create(GetJobStatsResponseSchema, {
      stats: [
        {
          jobId: 'steam',
          totalRuns: 10n,
          failedRuns: 2n,
          avgDurationMs: 1200n,
          lastRunAt: '2026-01-01T10:00:00Z'
        }
      ],
      recentRuns: [
        {
          jobId: 'steam',
          startedAt: '2026-01-01T10:00:00Z',
          durationMs: 1200n,
          success: false,
          error: 'steam api unreachable'
        }
      ]
    })

    render(<JobsCard data={data} />)
    // "steam" appears in both the stats table and the failures list.
    expect(screen.getAllByText('steam').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('steam api unreachable')).toBeInTheDocument()
    expect(screen.getByText('80%')).toBeInTheDocument()
  })

  it('shows an empty state without data', () => {
    render(<JobsCard data={undefined} />)
    expect(screen.getByText('No job runs recorded.')).toBeInTheDocument()
  })
})

describe('StorageCard', () => {
  it('flags orphaned objects for cleanup', () => {
    const data = create(GetStorageStatsResponseSchema, {
      latest: {
        scannedAt: '2026-01-01T00:00:00Z',
        totalSizeBytes: 1048576n,
        objectCount: 3n,
        orphanSizeBytes: 1024n,
        orphanCount: 1n,
        staleUploadSizeBytes: 0n,
        staleUploadCount: 0n,
        prefixBreakdown: [{ prefix: 'books', sizeBytes: 1048576n, count: 3n }]
      },
      history: [
        {
          scannedAt: '2026-01-01T00:00:00Z',
          totalSizeBytes: 1048576n,
          objectCount: 3n,
          orphanSizeBytes: 1024n,
          orphanCount: 1n,
          staleUploadSizeBytes: 0n,
          staleUploadCount: 0n,
          prefixBreakdown: []
        }
      ]
    })

    render(<StorageCard data={data} />)
    expect(screen.getByText(/orphaned/)).toBeInTheDocument()
    expect(screen.getByText('books')).toBeInTheDocument()
  })

  it('shows no-cleanup badge when clean', () => {
    const data = create(GetStorageStatsResponseSchema, {
      latest: {
        scannedAt: '2026-01-01T00:00:00Z',
        totalSizeBytes: 100n,
        objectCount: 1n,
        orphanSizeBytes: 0n,
        orphanCount: 0n,
        staleUploadSizeBytes: 0n,
        staleUploadCount: 0n,
        prefixBreakdown: []
      },
      history: []
    })

    render(<StorageCard data={data} />)
    expect(screen.getByText('No cleanup needed')).toBeInTheDocument()
  })
})

describe('DatabaseCard', () => {
  it('renders schema sizes', () => {
    const data = create(GetDatabaseStatsResponseSchema, {
      totalSizeBytes: 2097152n,
      schemas: [
        { name: 'books', sizeBytes: 1048576n, tableCount: 4n },
        { name: 'global', sizeBytes: 1048576n, tableCount: 3n }
      ]
    })

    render(<DatabaseCard data={data} />)
    expect(screen.getByText('books')).toBeInTheDocument()
    expect(screen.getByText('global')).toBeInTheDocument()
  })
})
