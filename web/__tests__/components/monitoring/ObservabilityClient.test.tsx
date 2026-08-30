import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  GetJobStatsResponseSchema,
  GetStorageStatsResponseSchema,
  GetDatabaseStatsResponseSchema,
  GetDatabaseSizeHistoryResponseSchema,
  GetSlowTransactionsResponseSchema,
  GetTransactionLatencyHistoryResponseSchema,
  GetHostMetricsResponseSchema,
  GetLogsResponseSchema,
  GetAlertStatesResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

const mockUseJobStats = jest.fn()
const mockUseStorageStats = jest.fn()
const mockTriggerStorageScan = jest.fn()
const mockUseDatabaseStats = jest.fn()
const mockUseDatabaseSizeHistory = jest.fn()
const mockUseSlowTransactions = jest.fn()
const mockUseTransactionLatencyHistory = jest.fn()
const mockUseHostMetrics = jest.fn()
const mockUseAlertStates = jest.fn()
const mockUseLogs = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useJobStats: (d: number) => mockUseJobStats(d),
  useStorageStats: () => mockUseStorageStats(),
  useTriggerStorageScan: () => mockTriggerStorageScan,
  useDatabaseStats: (d: number) => mockUseDatabaseStats(d),
  useDatabaseSizeHistory: (d: number) => mockUseDatabaseSizeHistory(d),
  useSlowTransactions: () => mockUseSlowTransactions(),
  useTransactionLatencyHistory: (d: number) => mockUseTransactionLatencyHistory(d),
  useHostMetrics: () => mockUseHostMetrics(),
  useAlertStates: () => mockUseAlertStates(),
  useLogs: () => mockUseLogs()
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
  mockUseDatabaseSizeHistory.mockReturnValue({
    data: create(GetDatabaseSizeHistoryResponseSchema, { points: [] }),
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
  mockUseTransactionLatencyHistory.mockReturnValue({
    data: create(GetTransactionLatencyHistoryResponseSchema, { points: [] }),
    mutate: mockMutate
  })
  mockUseHostMetrics.mockReturnValue({
    data: create(GetHostMetricsResponseSchema, {
      cpuPercent: 12.3,
      memoryPercent: 45.6,
      diskPercent: 78.9
    }),
    mutate: mockMutate
  })
  mockUseAlertStates.mockReturnValue({
    data: create(GetAlertStatesResponseSchema, { states: [] }),
    mutate: mockMutate
  })
  mockUseLogs.mockReturnValue({
    data: create(GetLogsResponseSchema, { entries: [] }),
    isLoading: false
  })
})

describe('ObservabilityClient', () => {
  it('renders a collapsed section per data source, with no headline tiles', () => {
    render(<ObservabilityClient />)
    expect(screen.getByText('Observability')).toBeInTheDocument()

    for (const title of [
      'Storage',
      'Database',
      'Database Size History',
      'Jobs',
      'Slow Transactions',
      'Transaction Latency History',
      'Host Metrics',
      'Logs'
    ]) {
      expect(screen.getByRole('button', { name: title })).toHaveAttribute('aria-expanded', 'false')
    }

    // Content is folded away until a section is expanded.
    expect(screen.queryByText(/orphaned/)).not.toBeInTheDocument()
  })

  it('expands a section to reveal its content and can be collapsed again', () => {
    render(<ObservabilityClient />)

    const storageToggle = screen.getByRole('button', { name: 'Storage' })
    fireEvent.click(storageToggle)
    expect(storageToggle).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText(/orphaned/)).toBeInTheDocument()

    fireEvent.click(storageToggle)
    expect(storageToggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText(/orphaned/)).not.toBeInTheDocument()
  })

  it('refetches job and database stats when the window changes', () => {
    render(<ObservabilityClient />)
    expect(mockUseJobStats).toHaveBeenCalledWith(30)
    expect(mockUseDatabaseStats).toHaveBeenCalledWith(30)

    fireEvent.change(screen.getByLabelText('Time window'), { target: { value: '7' } })
    expect(mockUseJobStats).toHaveBeenCalledWith(7)
    expect(mockUseDatabaseStats).toHaveBeenCalledWith(7)
  })

  it('links to the monitoring settings page and back to issues', () => {
    render(<ObservabilityClient />)
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute(
      'href',
      '/monitoring/settings'
    )
    expect(screen.getByRole('link', { name: 'Back to monitoring' })).toHaveAttribute(
      'href',
      '/monitoring'
    )
  })

  it('revalidates every data source when Refresh is clicked', async () => {
    render(<ObservabilityClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))

    expect(screen.getByRole('button', { name: 'Refreshing…' })).toBeDisabled()
    // storageStats is refreshed via triggerStorageScan (a live R2 rescan)
    // instead of a plain mutate(), so mockMutate covers the other 6 sources.
    expect(mockMutate).toHaveBeenCalledTimes(6)
    expect(mockTriggerStorageScan).toHaveBeenCalledTimes(1)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh' })).not.toBeDisabled())
  })
})
