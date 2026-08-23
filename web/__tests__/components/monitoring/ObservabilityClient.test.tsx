import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  GetJobStatsResponseSchema,
  GetStorageStatsResponseSchema,
  GetDatabaseStatsResponseSchema,
  GetFailingPullRequestsResponseSchema,
  GetWorkflowRunsResponseSchema,
  GetSecurityAlertsResponseSchema,
  GetSentryIssuesResponseSchema,
  GetSlowTransactionsResponseSchema,
  GetDeployStatusResponseSchema,
  ListOAuthConnectionsResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

const mockUseJobStats = jest.fn()
const mockUseUsageStats = jest.fn()
const mockUseStorageStats = jest.fn()
const mockTriggerStorageScan = jest.fn()
const mockUseDatabaseStats = jest.fn()
const mockUseFailingPullRequests = jest.fn()
const mockUseWorkflowRuns = jest.fn()
const mockUseSecurityAlerts = jest.fn()
const mockUseSentryIssues = jest.fn()
const mockUseSlowTransactions = jest.fn()
const mockUseDeployStatus = jest.fn()
const mockUseOAuthConnections = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useJobStats: (d: number) => mockUseJobStats(d),
  useUsageStats: (d: number) => mockUseUsageStats(d),
  useStorageStats: () => mockUseStorageStats(),
  useTriggerStorageScan: () => mockTriggerStorageScan,
  useDatabaseStats: () => mockUseDatabaseStats(),
  useFailingPullRequests: () => mockUseFailingPullRequests(),
  useWorkflowRuns: () => mockUseWorkflowRuns(),
  useSecurityAlerts: () => mockUseSecurityAlerts(),
  useSentryIssues: () => mockUseSentryIssues(),
  useSlowTransactions: () => mockUseSlowTransactions(),
  useDeployStatus: () => mockUseDeployStatus(),
  useOAuthConnections: () => mockUseOAuthConnections(),
  useDisconnectOAuthConnection: () => jest.fn()
}))

jest.mock('recharts', () => {
  const Original = jest.requireActual('recharts')
  return {
    ...Original,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 400, height: 300 }}>{children}</div>
    )
  }
})

const mockMutate = jest.fn()

beforeEach(() => {
  jest.clearAllMocks()
  mockMutate.mockResolvedValue(undefined)
  mockTriggerStorageScan.mockResolvedValue(undefined)
  mockUseJobStats.mockReturnValue({
    data: create(GetJobStatsResponseSchema, { stats: [], recentRuns: [] }),
    mutate: mockMutate
  })
  mockUseUsageStats.mockReturnValue({ data: undefined, mutate: mockMutate })
  mockUseStorageStats.mockReturnValue({
    data: create(GetStorageStatsResponseSchema, {
      latest: {
        scannedAt: '2026-01-01T00:00:00Z',
        totalSizeBytes: 1048576n,
        objectCount: 3n,
        orphanSizeBytes: 2048n,
        orphanCount: 1n,
        staleUploadSizeBytes: 0n,
        staleUploadCount: 0n,
        prefixBreakdown: []
      },
      history: []
    }),
    mutate: mockMutate
  })
  mockUseDatabaseStats.mockReturnValue({
    data: create(GetDatabaseStatsResponseSchema, { totalSizeBytes: 2097152n, schemas: [] }),
    mutate: mockMutate
  })
  mockUseFailingPullRequests.mockReturnValue({
    data: create(GetFailingPullRequestsResponseSchema, {
      configured: true,
      failingCount: 2,
      pullRequests: []
    }),
    mutate: mockMutate
  })
  mockUseWorkflowRuns.mockReturnValue({
    data: create(GetWorkflowRunsResponseSchema, { configured: true, runs: [] }),
    mutate: mockMutate
  })
  mockUseSecurityAlerts.mockReturnValue({
    data: create(GetSecurityAlertsResponseSchema, {
      configured: true,
      alertCount: 0,
      alerts: []
    }),
    mutate: mockMutate
  })
  mockUseSentryIssues.mockReturnValue({
    data: create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 0,
      issues: []
    }),
    mutate: mockMutate
  })
  mockUseSlowTransactions.mockReturnValue({
    data: create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [],
      trending: []
    }),
    mutate: mockMutate
  })
  mockUseDeployStatus.mockReturnValue({
    data: create(GetDeployStatusResponseSchema, { configured: true, phase: 'ACTIVE' }),
    mutate: mockMutate
  })
  mockUseOAuthConnections.mockReturnValue({
    data: create(ListOAuthConnectionsResponseSchema, { connections: [] }),
    mutate: mockMutate
  })
})

describe('ObservabilityClient', () => {
  it('renders the headline tiles from hook data', () => {
    render(<ObservabilityClient />)
    expect(screen.getByText('Observability')).toBeInTheDocument()
    expect(screen.getByText('R2 storage')).toBeInTheDocument()
    expect(screen.getByText('Database')).toBeInTheDocument()
    // Orphaned bytes tile reflects the snapshot.
    expect(screen.getByText('2.0 KB')).toBeInTheDocument()
    // External-signal tiles render from their hook data.
    expect(screen.getByText('Failing PRs')).toBeInTheDocument()
    expect(screen.getByText('Unresolved errors')).toBeInTheDocument()
    expect(screen.getByText('Deploy')).toBeInTheDocument()
    expect(screen.getByText('ACTIVE')).toBeInTheDocument()
  })

  it('degrades external tiles when their sources are unconfigured', () => {
    mockUseFailingPullRequests.mockReturnValue({
      data: create(GetFailingPullRequestsResponseSchema, { configured: false })
    })
    mockUseSecurityAlerts.mockReturnValue({
      data: create(GetSecurityAlertsResponseSchema, { configured: false })
    })
    mockUseSentryIssues.mockReturnValue({
      data: create(GetSentryIssuesResponseSchema, { configured: false })
    })
    mockUseDeployStatus.mockReturnValue({
      data: create(GetDeployStatusResponseSchema, { configured: false })
    })

    render(<ObservabilityClient />)
    // Failing PRs / Security alerts / Unresolved errors / Deploy tiles all
    // fall back to a dash.
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(4)
  })

  it('refetches job/usage stats when the window changes', () => {
    render(<ObservabilityClient />)
    expect(mockUseJobStats).toHaveBeenCalledWith(30)

    fireEvent.change(screen.getByLabelText('Time window'), { target: { value: '7' } })
    expect(mockUseJobStats).toHaveBeenCalledWith(7)
    expect(mockUseUsageStats).toHaveBeenCalledWith(7)
  })

  it('revalidates every data source when Refresh is clicked', async () => {
    render(<ObservabilityClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))

    expect(screen.getByRole('button', { name: 'Refreshing…' })).toBeDisabled()
    // storageStats is refreshed via triggerStorageScan (a live R2 rescan)
    // instead of a plain mutate(), so mockMutate covers the other 10 sources.
    expect(mockMutate).toHaveBeenCalledTimes(10)
    expect(mockTriggerStorageScan).toHaveBeenCalledTimes(1)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh' })).not.toBeDisabled())
  })
})
