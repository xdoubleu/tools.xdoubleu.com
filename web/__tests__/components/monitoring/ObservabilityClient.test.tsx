import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent } from '@testing-library/react'
import {
  GetJobStatsResponseSchema,
  GetStorageStatsResponseSchema,
  GetDatabaseStatsResponseSchema,
  GetGithubIssuesResponseSchema,
  GetSentryIssuesResponseSchema,
  GetDeployStatusResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

const mockUseJobStats = jest.fn()
const mockUseUsageStats = jest.fn()
const mockUseStorageStats = jest.fn()
const mockUseDatabaseStats = jest.fn()
const mockUseGithubIssues = jest.fn()
const mockUseSentryIssues = jest.fn()
const mockUseDeployStatus = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useJobStats: (d: number) => mockUseJobStats(d),
  useUsageStats: (d: number) => mockUseUsageStats(d),
  useStorageStats: () => mockUseStorageStats(),
  useDatabaseStats: () => mockUseDatabaseStats(),
  useGithubIssues: () => mockUseGithubIssues(),
  useSentryIssues: () => mockUseSentryIssues(),
  useDeployStatus: () => mockUseDeployStatus()
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

beforeEach(() => {
  jest.clearAllMocks()
  mockUseJobStats.mockReturnValue({
    data: create(GetJobStatsResponseSchema, { stats: [], recentRuns: [] })
  })
  mockUseUsageStats.mockReturnValue({ data: undefined })
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
    })
  })
  mockUseDatabaseStats.mockReturnValue({
    data: create(GetDatabaseStatsResponseSchema, { totalSizeBytes: 2097152n, schemas: [] })
  })
  mockUseGithubIssues.mockReturnValue({
    data: create(GetGithubIssuesResponseSchema, { configured: true, openCount: 3, issues: [] })
  })
  mockUseSentryIssues.mockReturnValue({
    data: create(GetSentryIssuesResponseSchema, {
      configured: true,
      unresolvedCount: 0,
      issues: []
    })
  })
  mockUseDeployStatus.mockReturnValue({
    data: create(GetDeployStatusResponseSchema, { configured: true, phase: 'ACTIVE' })
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
    expect(screen.getByText('Open issues')).toBeInTheDocument()
    expect(screen.getByText('Unresolved errors')).toBeInTheDocument()
    expect(screen.getByText('Deploy')).toBeInTheDocument()
    expect(screen.getByText('ACTIVE')).toBeInTheDocument()
  })

  it('degrades external tiles when their sources are unconfigured', () => {
    mockUseGithubIssues.mockReturnValue({
      data: create(GetGithubIssuesResponseSchema, { configured: false })
    })
    mockUseSentryIssues.mockReturnValue({
      data: create(GetSentryIssuesResponseSchema, { configured: false })
    })
    mockUseDeployStatus.mockReturnValue({
      data: create(GetDeployStatusResponseSchema, { configured: false })
    })

    render(<ObservabilityClient />)
    // Open issues / Unresolved errors / Deploy tiles all fall back to a dash.
    expect(screen.getAllByText('—').length).toBeGreaterThanOrEqual(3)
  })

  it('refetches job/usage stats when the window changes', () => {
    render(<ObservabilityClient />)
    expect(mockUseJobStats).toHaveBeenCalledWith(30)

    fireEvent.change(screen.getByLabelText('Time window'), { target: { value: '7' } })
    expect(mockUseJobStats).toHaveBeenCalledWith(7)
    expect(mockUseUsageStats).toHaveBeenCalledWith(7)
  })
})
