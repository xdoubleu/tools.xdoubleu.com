import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  GetFailingPullRequestsResponseSchema,
  GetWorkflowRunsResponseSchema,
  GetSecurityAlertsResponseSchema,
  GetSentryIssuesResponseSchema,
  GetStorageStatsResponseSchema,
  GetAlertStatesResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import IssuesClient from '@/components/monitoring/IssuesClient'

const mockUseFailingPullRequests = jest.fn()
const mockUseWorkflowRuns = jest.fn()
const mockUseSecurityAlerts = jest.fn()
const mockUseSentryIssues = jest.fn()
const mockUseStorageStats = jest.fn()
const mockUseAlertStates = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useFailingPullRequests: () => mockUseFailingPullRequests(),
  useWorkflowRuns: () => mockUseWorkflowRuns(),
  useSecurityAlerts: () => mockUseSecurityAlerts(),
  useSentryIssues: () => mockUseSentryIssues(),
  useStorageStats: () => mockUseStorageStats(),
  useAlertStates: () => mockUseAlertStates(),
  useResolveSentryIssue: () => jest.fn()
}))

const mockMutate = jest.fn()

beforeEach(() => {
  jest.clearAllMocks()
  mockMutate.mockResolvedValue(undefined)
  mockUseFailingPullRequests.mockReturnValue({
    data: create(GetFailingPullRequestsResponseSchema, {
      configured: true,
      failingCount: 2,
      pullRequests: []
    }),
    mutate: mockMutate
  })
  mockUseWorkflowRuns.mockReturnValue({
    data: create(GetWorkflowRunsResponseSchema, {
      configured: true,
      runs: [
        {
          id: 1n,
          name: 'CI',
          event: 'pull_request',
          branch: 'feat/x',
          status: 'completed',
          conclusion: 'failure',
          url: 'https://github.com/x/y/actions/runs/1',
          startedAt: '2026-01-01T10:00:00Z',
          durationMs: 60000n
        },
        {
          id: 2n,
          name: 'CI',
          event: 'push',
          branch: 'main',
          status: 'completed',
          conclusion: 'failure',
          url: 'https://github.com/x/y/actions/runs/2',
          startedAt: '2026-01-01T11:00:00Z',
          durationMs: 60000n
        }
      ]
    }),
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
  mockUseStorageStats.mockReturnValue({
    data: create(GetStorageStatsResponseSchema, {
      latest: {
        scannedAt: '2026-08-27T14:13:28Z',
        totalSizeBytes: 100n,
        objectCount: 10n,
        orphanSizeBytes: 0n,
        orphanCount: 0n,
        staleUploadSizeBytes: 0n,
        staleUploadCount: 0n,
        prefixBreakdown: [],
        orphanKeys: []
      },
      history: []
    }),
    mutate: mockMutate
  })
  mockUseAlertStates.mockReturnValue({
    data: create(GetAlertStatesResponseSchema, {
      states: [
        { ruleKey: 'host_cpu_high', breaching: false, currentValue: 12, threshold: 80 },
        { ruleKey: 'host_disk_high', breaching: true, currentValue: 91, threshold: 85 }
      ]
    }),
    mutate: mockMutate
  })
})

describe('IssuesClient', () => {
  it('renders the headline tiles from hook data', () => {
    render(<IssuesClient />)
    expect(screen.getByText('Issues')).toBeInTheDocument()
    expect(screen.getByText('Failing dependency PRs')).toBeInTheDocument()
    expect(screen.getByText('Unresolved errors')).toBeInTheDocument()
    expect(screen.getByText('Breaching alerts')).toBeInTheDocument()
    expect(screen.getByText('Threshold alerts')).toBeInTheDocument()
  })

  it('only counts push runs on main with a failing conclusion', () => {
    render(<IssuesClient />)
    // Two runs total, one PR run (excluded) and one main push failure
    // (included) -> tile should read 1.
    expect(screen.getAllByText('Failing runs on main').length).toBeGreaterThan(0)
    expect(screen.getAllByText('1').length).toBeGreaterThan(0)
    // Only the main failing run is rendered in the card, not the PR run.
    expect(screen.queryByText('feat/x')).not.toBeInTheDocument()
    expect(screen.getAllByText('main').length).toBeGreaterThan(0)
  })

  it('degrades tiles when unconfigured and flags danger tones when counts are positive', () => {
    mockUseFailingPullRequests.mockReturnValue({
      data: create(GetFailingPullRequestsResponseSchema, { configured: false }),
      mutate: mockMutate
    })
    mockUseSecurityAlerts.mockReturnValue({
      data: create(GetSecurityAlertsResponseSchema, {
        configured: true,
        alertCount: 3,
        alerts: []
      }),
      mutate: mockMutate
    })
    mockUseSentryIssues.mockReturnValue({
      data: create(GetSentryIssuesResponseSchema, {
        configured: true,
        unresolvedCount: 5,
        issues: []
      }),
      mutate: mockMutate
    })
    mockUseWorkflowRuns.mockReturnValue({ data: undefined, mutate: mockMutate })

    render(<IssuesClient />)
    expect(screen.getAllByText('—').length).toBeGreaterThan(0)
    expect(screen.getAllByText('3').length).toBeGreaterThan(0)
    expect(screen.getAllByText('5').length).toBeGreaterThan(0)
  })

  it('shows a placeholder breaching-alerts tile until alert states load', () => {
    mockUseAlertStates.mockReturnValue({ data: undefined, mutate: mockMutate })

    render(<IssuesClient />)
    expect(screen.getByText('Breaching alerts')).toBeInTheDocument()
    expect(screen.getAllByText('—').length).toBeGreaterThan(0)
  })

  it('counts every breaching rule in the breaching-alerts tile', () => {
    mockUseAlertStates.mockReturnValue({
      data: create(GetAlertStatesResponseSchema, {
        states: [
          { ruleKey: 'host_cpu_high', breaching: true, currentValue: 95, threshold: 80 },
          { ruleKey: 'host_memory_high', breaching: true, currentValue: 92, threshold: 85 },
          { ruleKey: 'host_disk_high', breaching: false, currentValue: 12, threshold: 85 }
        ]
      }),
      mutate: mockMutate
    })

    render(<IssuesClient />)
    expect(screen.getByText('2 breaching')).toBeInTheDocument()
  })

  it('links to the observability and monitoring settings pages', () => {
    render(<IssuesClient />)
    expect(screen.getByRole('link', { name: 'Observability' })).toHaveAttribute(
      'href',
      '/monitoring/observability'
    )
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute(
      'href',
      '/monitoring/settings'
    )
  })

  it('revalidates every data source when Refresh is clicked', async () => {
    render(<IssuesClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))

    expect(screen.getByRole('button', { name: 'Refreshing…' })).toBeDisabled()
    expect(mockMutate).toHaveBeenCalledTimes(6)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh' })).not.toBeDisabled())
  })

  it('flags orphaned storage danger tone and lists orphan keys', () => {
    mockUseStorageStats.mockReturnValue({
      data: create(GetStorageStatsResponseSchema, {
        latest: {
          scannedAt: '2026-08-27T14:13:28Z',
          totalSizeBytes: 100n,
          objectCount: 10n,
          orphanSizeBytes: 202671n,
          orphanCount: 2n,
          staleUploadSizeBytes: 0n,
          staleUploadCount: 0n,
          prefixBreakdown: [],
          orphanKeys: ['books/x/y.epub', 'books/a/b.epub']
        },
        history: []
      }),
      mutate: mockMutate
    })

    render(<IssuesClient />)

    expect(screen.getAllByText('Orphaned storage').length).toBeGreaterThan(0)
    expect(screen.getAllByText('2').length).toBeGreaterThan(0)
    expect(screen.getByText('books/x/y.epub')).toBeInTheDocument()
    expect(screen.getByText('books/a/b.epub')).toBeInTheDocument()
  })
})
