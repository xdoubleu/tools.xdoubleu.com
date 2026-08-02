import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  GetJobStatsResponseSchema,
  GetStorageStatsResponseSchema,
  GetDatabaseStatsResponseSchema,
  GetFailingPullRequestsResponseSchema,
  GetSentryIssuesResponseSchema,
  GetDeployStatusResponseSchema,
  DeployComponentLogSchema
} from '@/lib/gen/observability/v1/observability_pb'
import JobsCard from '@/components/monitoring/JobsCard'
import StorageCard from '@/components/monitoring/StorageCard'
import DatabaseCard from '@/components/monitoring/DatabaseCard'
import FailingPullRequestsCard from '@/components/monitoring/FailingPullRequestsCard'
import SentryCard from '@/components/monitoring/SentryCard'
import DeployCard from '@/components/monitoring/DeployCard'

const mockGetDeployLogs = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useDeployLogs: () => mockGetDeployLogs
}))

beforeEach(() => {
  mockGetDeployLogs.mockReset()
})

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

describe('FailingPullRequestsCard', () => {
  it('renders failing pull requests with their checks', () => {
    const data = create(GetFailingPullRequestsResponseSchema, {
      configured: true,
      failingCount: 1,
      pullRequests: [
        {
          number: 12n,
          title: 'Broken build',
          url: 'https://github.com/x/y/pull/12',
          author: 'alice',
          updatedAt: '2026-01-01T00:00:00Z',
          failingChecks: [{ name: 'ci-pass', conclusion: 'failure', url: 'https://gh/checks/1' }]
        }
      ]
    })

    render(<FailingPullRequestsCard data={data} />)
    expect(screen.getByText('Broken build')).toBeInTheDocument()
    expect(screen.getByText('#12')).toBeInTheDocument()
    expect(screen.getByText('ci-pass')).toBeInTheDocument()
    expect(screen.getByText(/alice/)).toBeInTheDocument()
  })

  it('degrades when not configured', () => {
    const data = create(GetFailingPullRequestsResponseSchema, { configured: false })
    render(<FailingPullRequestsCard data={data} />)
    expect(screen.getByText('GitHub is not configured.')).toBeInTheDocument()
  })

  it('shows an empty state when configured with no failing pull requests', () => {
    const data = create(GetFailingPullRequestsResponseSchema, {
      configured: true,
      failingCount: 0
    })
    render(<FailingPullRequestsCard data={data} />)
    expect(screen.getByText('No failing pull requests.')).toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<FailingPullRequestsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})

describe('SentryCard', () => {
  it('renders unresolved issues with level badge', () => {
    const data = create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 1,
      issues: [
        {
          id: 'abc',
          title: 'TypeError: boom',
          culprit: 'app/foo.ts',
          permalink: 'https://sentry.io/x',
          count: 12n,
          lastSeen: '2026-01-01T00:00:00Z',
          level: 'error'
        }
      ]
    })

    render(<SentryCard data={data} />)
    expect(screen.getByText('TypeError: boom')).toBeInTheDocument()
    expect(screen.getByText('app/foo.ts')).toBeInTheDocument()
    expect(screen.getByText('error')).toBeInTheDocument()
    expect(screen.getByText('12 events')).toBeInTheDocument()
  })

  it('maps warning and info levels to their badge variants', () => {
    const data = create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 2,
      issues: [
        {
          id: 'warn',
          title: 'A warning',
          permalink: 'https://sentry.io/w',
          count: 1n,
          lastSeen: '2026-01-01T00:00:00Z',
          level: 'warning'
        },
        {
          id: 'info',
          title: 'An info',
          permalink: 'https://sentry.io/i',
          count: 1n,
          lastSeen: '2026-01-01T00:00:00Z',
          level: 'info'
        }
      ]
    })

    render(<SentryCard data={data} />)
    expect(screen.getByText('warning')).toBeInTheDocument()
    expect(screen.getByText('info')).toBeInTheDocument()
  })

  it('degrades when not configured', () => {
    const data = create(GetSentryIssuesResponseSchema, { configured: false })
    render(<SentryCard data={data} />)
    expect(screen.getByText('Sentry is not configured.')).toBeInTheDocument()
  })

  it('shows an empty state when configured with no issues', () => {
    const data = create(GetSentryIssuesResponseSchema, { configured: true, unresolvedCount: 0 })
    render(<SentryCard data={data} />)
    expect(screen.getByText('No unresolved issues.')).toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<SentryCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})

describe('DeployCard', () => {
  it('renders the latest deployment', () => {
    const data = create(GetDeployStatusResponseSchema, {
      configured: true,
      phase: 'ACTIVE',
      cause: 'manual deploy',
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:05:00Z',
      deploymentId: 'deploy-123'
    })

    render(<DeployCard data={data} />)
    expect(screen.getByText('ACTIVE')).toBeInTheDocument()
    expect(screen.getByText('manual deploy')).toBeInTheDocument()
    expect(screen.getByText('deploy-123')).toBeInTheDocument()
  })

  it('flags a failed deployment phase', () => {
    const data = create(GetDeployStatusResponseSchema, {
      configured: true,
      phase: 'ERROR',
      deploymentId: 'deploy-err'
    })
    render(<DeployCard data={data} />)
    expect(screen.getByText('ERROR')).toBeInTheDocument()
  })

  it('renders an in-progress deployment phase', () => {
    const data = create(GetDeployStatusResponseSchema, {
      configured: true,
      phase: 'BUILDING',
      deploymentId: 'deploy-wip'
    })
    render(<DeployCard data={data} />)
    expect(screen.getByText('BUILDING')).toBeInTheDocument()
  })

  it('degrades when not configured', () => {
    const data = create(GetDeployStatusResponseSchema, { configured: false })
    render(<DeployCard data={data} />)
    expect(screen.getByText('DigitalOcean is not configured.')).toBeInTheDocument()
  })

  it('shows an empty state when configured without a deployment', () => {
    const data = create(GetDeployStatusResponseSchema, { configured: true, deploymentId: '' })
    render(<DeployCard data={data} />)
    expect(screen.getByText('No deployment recorded.')).toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<DeployCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('opens the logs dialog and fetches logs for the deployment', async () => {
    mockGetDeployLogs.mockReturnValue({
      async *[Symbol.asyncIterator]() {
        yield create(DeployComponentLogSchema, {
          component: 'api',
          logType: 'BUILD',
          content: 'building api\n',
          truncated: false
        })
      }
    })
    const data = create(GetDeployStatusResponseSchema, {
      configured: true,
      phase: 'ACTIVE',
      deploymentId: 'deploy-123'
    })

    render(<DeployCard data={data} />)
    fireEvent.click(screen.getByRole('button', { name: 'View logs' }))

    expect(mockGetDeployLogs).toHaveBeenCalledWith('deploy-123')
    await waitFor(() => expect(screen.getByText('building api')).toBeInTheDocument())
    expect(screen.getByText('BUILD')).toBeInTheDocument()
  })
})
