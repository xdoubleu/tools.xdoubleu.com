import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import { formatDateTime } from '@/lib/dates'
import {
  GetJobStatsResponseSchema,
  GetStorageStatsResponseSchema,
  GetDatabaseStatsResponseSchema,
  GetFailingPullRequestsResponseSchema,
  GetWorkflowRunsResponseSchema,
  GetSecurityAlertsResponseSchema,
  GetSentryIssuesResponseSchema,
  GetHostMetricsResponseSchema,
  SecurityAlertType
} from '@/lib/gen/observability/v1/observability_pb'
import JobsCard from '@/components/monitoring/JobsCard'
import StorageCard from '@/components/monitoring/StorageCard'
import DatabaseCard from '@/components/monitoring/DatabaseCard'
import FailingPullRequestsCard from '@/components/monitoring/FailingPullRequestsCard'
import WorkflowRunsCard from '@/components/monitoring/WorkflowRunsCard'
import SecurityAlertsCard from '@/components/monitoring/SecurityAlertsCard'
import SentryCard from '@/components/monitoring/SentryCard'
import OrphanedStorageCard from '@/components/monitoring/OrphanedStorageCard'
import HostMetricsCard, {
  xAxisTickFormatter,
  yAxisTickFormatter,
  tooltipLabelFormatter,
  tooltipValueFormatter
} from '@/components/monitoring/HostMetricsCard'

const mockResolveSentryIssue = jest.fn()
jest.mock('@/hooks/useMonitoring', () => ({
  useResolveSentryIssue: () => mockResolveSentryIssue
}))

beforeEach(() => {
  mockResolveSentryIssue.mockReset()
  mockResolveSentryIssue.mockResolvedValue(undefined)
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
        prefixBreakdown: [{ prefix: 'books', sizeBytes: 1048576n, count: 3n }],
        orphanKeys: ['books/b1/orphan.epub']
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
    expect(screen.getByText('books/b1/orphan.epub')).toBeInTheDocument()
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

describe('WorkflowRunsCard', () => {
  it('renders completed and in-progress runs with duration', () => {
    const data = create(GetWorkflowRunsResponseSchema, {
      configured: true,
      runs: [
        {
          id: 1n,
          name: 'CI',
          event: 'pull_request',
          branch: 'feat/x',
          status: 'completed',
          conclusion: 'success',
          url: 'https://github.com/x/y/actions/runs/1',
          startedAt: '2026-01-01T10:00:00Z',
          durationMs: 300000n
        },
        {
          id: 2n,
          name: 'CI',
          event: 'push',
          branch: 'release-branch',
          status: 'in_progress',
          conclusion: '',
          url: 'https://github.com/x/y/actions/runs/2',
          startedAt: '2026-01-01T11:00:00Z',
          durationMs: 0n
        },
        {
          id: 3n,
          name: 'CI',
          event: 'push',
          branch: 'main',
          status: 'completed',
          conclusion: 'failure',
          url: 'https://github.com/x/y/actions/runs/3',
          startedAt: '2026-01-01T12:00:00Z',
          durationMs: 60000n
        }
      ]
    })

    render(<WorkflowRunsCard data={data} />)
    expect(screen.getByText('PR')).toBeInTheDocument()
    expect(screen.getByText('release-branch')).toBeInTheDocument()
    expect(screen.getByText('5.0 min')).toBeInTheDocument()
    expect(screen.getByText('in_progress')).toBeInTheDocument()
    expect(screen.getByText('failure')).toBeInTheDocument()
    expect(screen.getByText('1.0 min')).toBeInTheDocument()
  })

  it('degrades when not configured', () => {
    const data = create(GetWorkflowRunsResponseSchema, { configured: false })
    render(<WorkflowRunsCard data={data} />)
    expect(screen.getByText('GitHub is not configured.')).toBeInTheDocument()
  })

  it('shows an empty state when configured with no runs', () => {
    const data = create(GetWorkflowRunsResponseSchema, { configured: true, runs: [] })
    render(<WorkflowRunsCard data={data} />)
    expect(screen.getByText('No workflow runs.')).toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<WorkflowRunsCard data={undefined} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })
})

describe('SecurityAlertsCard', () => {
  it('renders open security alerts with severity badge', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [
        {
          number: 83n,
          alertType: SecurityAlertType.DEPENDABOT,
          packageName: 'otel',
          ecosystem: 'go',
          severity: 'unmapped-severity',
          summary: 'unbounded body read',
          url: 'https://github.com/x/y/security/dependabot/83',
          createdAt: '2026-08-19T16:34:44Z'
        }
      ]
    })

    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('otel')).toBeInTheDocument()
    expect(screen.getByText('#83')).toBeInTheDocument()
    expect(screen.getByText('unbounded body read')).toBeInTheDocument()
    expect(screen.getByText('go ·', { exact: false })).toBeInTheDocument()
    expect(screen.getByText('unmapped-severity')).toBeInTheDocument()
  })

  it('renders a code scanning alert with file location', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [
        {
          number: 12n,
          alertType: SecurityAlertType.CODE_SCANNING,
          ruleId: 'go/sql-injection',
          filePath: 'api/foo.go',
          line: 42,
          severity: 'high',
          summary: 'SQL injection',
          url: 'https://github.com/x/y/security/code-scanning/12',
          createdAt: '2026-08-20T10:00:00Z'
        }
      ]
    })

    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('api/foo.go:42')).toBeInTheDocument()
    expect(screen.getByText('Code scanning')).toBeInTheDocument()
  })

  it('falls back to the rule id when a code scanning alert has no file path', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [
        {
          number: 13n,
          alertType: SecurityAlertType.CODE_SCANNING,
          ruleId: 'go/sql-injection',
          severity: 'medium',
          url: 'https://github.com/x/y/security/code-scanning/13',
          createdAt: '2026-08-20T10:00:00Z'
        }
      ]
    })

    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('go/sql-injection')).toBeInTheDocument()
    expect(screen.getByText('medium')).toBeInTheDocument()
  })

  it('renders a secret scanning alert with its secret type', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [
        {
          number: 7n,
          alertType: SecurityAlertType.SECRET_SCANNING,
          secretType: 'AWS Access Key',
          url: 'https://github.com/x/y/security/secret-scanning/7',
          createdAt: '2026-08-21T09:00:00Z'
        }
      ]
    })

    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('AWS Access Key')).toBeInTheDocument()
    expect(screen.getByText('Secret scanning')).toBeInTheDocument()
  })

  it('omits the ecosystem prefix when a dependabot alert has none', () => {
    const data = create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 1,
      alerts: [
        {
          number: 99n,
          packageName: 'lodash',
          url: 'https://github.com/x/y/security/dependabot/99',
          createdAt: '2026-08-19T16:34:44Z'
        }
      ]
    })

    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('lodash')).toBeInTheDocument()
    expect(screen.queryByText(/·/)).not.toBeInTheDocument()
  })

  it('degrades when not configured', () => {
    const data = create(GetSecurityAlertsResponseSchema, { configured: false })
    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('GitHub is not configured.')).toBeInTheDocument()
  })

  it('shows an empty state when configured with no alerts', () => {
    const data = create(GetSecurityAlertsResponseSchema, { configured: true, alertCount: 0 })
    render(<SecurityAlertsCard data={data} />)
    expect(screen.getByText('No open security alerts.')).toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<SecurityAlertsCard data={undefined} />)
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

describe('OrphanedStorageCard', () => {
  it('renders orphaned object keys with a truncation note', () => {
    const data = create(GetStorageStatsResponseSchema, {
      latest: {
        scannedAt: '2026-08-27T14:13:28Z',
        totalSizeBytes: 1999408603n,
        objectCount: 1049n,
        orphanSizeBytes: 202671n,
        orphanCount: 2n,
        staleUploadSizeBytes: 0n,
        staleUploadCount: 0n,
        prefixBreakdown: [],
        orphanKeys: [
          'books/210892c5-8e27-4125-89de-935a2849ee6b/3e670514.epub',
          'books/a72f8384-3e1a-4aa6-8fb5-4e6e29b7d08f/b7abf2a3.epub'
        ]
      },
      history: []
    })

    render(<OrphanedStorageCard data={data} />)
    expect(
      screen.getByText('books/210892c5-8e27-4125-89de-935a2849ee6b/3e670514.epub')
    ).toBeInTheDocument()
    expect(
      screen.getByText('books/a72f8384-3e1a-4aa6-8fb5-4e6e29b7d08f/b7abf2a3.epub')
    ).toBeInTheDocument()
  })

  it('shows an empty state when there are no orphans', () => {
    const data = create(GetStorageStatsResponseSchema, {
      latest: {
        scannedAt: '2026-08-27T14:13:28Z',
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

    render(<OrphanedStorageCard data={data} />)
    expect(screen.getByText('No orphaned storage objects.')).toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<OrphanedStorageCard data={undefined} />)
    expect(screen.getByText('No scan recorded yet.')).toBeInTheDocument()
  })
})

describe('HostMetricsCard', () => {
  it('renders headline tiles and history charts', () => {
    const data = create(GetHostMetricsResponseSchema, {
      cpuPercent: 12.3,
      memoryPercent: 45.6,
      diskPercent: 78.9,
      cpuHistory: [{ timestamp: '2026-01-01T00:00:00Z', value: 10 }],
      memoryHistory: [{ timestamp: '2026-01-01T00:00:00Z', value: 40 }],
      diskHistory: [{ timestamp: '2026-01-01T00:00:00Z', value: 70 }]
    })

    render(<HostMetricsCard data={data} />)
    expect(screen.getByText('12.3%')).toBeInTheDocument()
    expect(screen.getByText('45.6%')).toBeInTheDocument()
    expect(screen.getByText('78.9%')).toBeInTheDocument()
  })

  it('shows a loading state without data', () => {
    render(<HostMetricsCard data={undefined} />)
    expect(screen.getAllByText('—').length).toBe(3)
  })

  it('shows a placeholder when a metric has no history yet', () => {
    const data = create(GetHostMetricsResponseSchema, {
      cpuPercent: 1,
      memoryPercent: 2,
      diskPercent: 3,
      cpuHistory: [],
      memoryHistory: [],
      diskHistory: []
    })

    render(<HostMetricsCard data={data} />)
    expect(screen.getAllByText('No history yet.').length).toBe(3)
  })
})

describe('HostMetricsCard chart formatters', () => {
  it('formats the x-axis tick as a time', () => {
    expect(xAxisTickFormatter('2026-01-01T13:45:00Z')).not.toBe('2026-01-01T13:45:00Z')
    expect(xAxisTickFormatter('not-a-date')).toBe('not-a-date')
  })

  it('formats the y-axis tick as a percentage', () => {
    expect(yAxisTickFormatter(42)).toBe('42%')
  })

  it('formats the tooltip label from a string timestamp', () => {
    expect(tooltipLabelFormatter('2026-01-01T13:45:00Z')).not.toBe('')
    expect(tooltipLabelFormatter(123)).toBe(formatDateTime(''))
  })

  it('formats the tooltip value as a percentage with its label', () => {
    expect(tooltipValueFormatter(12.34, 'CPU')).toEqual(['12.3%', 'CPU'])
  })
})
