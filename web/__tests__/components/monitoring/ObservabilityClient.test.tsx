import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import {
  GetJobStatsResponseSchema,
  GetStorageStatsResponseSchema,
  GetDatabaseStatsResponseSchema,
  GetSlowTransactionsResponseSchema,
  GetHostMetricsResponseSchema,
  GetLogsResponseSchema
} from '@/lib/gen/observability/v1/observability_pb'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

const mockUseJobStats = jest.fn()
const mockUseUsageStats = jest.fn()
const mockUseStorageStats = jest.fn()
const mockTriggerStorageScan = jest.fn()
const mockUseDatabaseStats = jest.fn()
const mockUseSlowTransactions = jest.fn()
const mockUseHostMetrics = jest.fn()
const mockUseLogs = jest.fn()

jest.mock('@/hooks/useMonitoring', () => ({
  useJobStats: (d: number) => mockUseJobStats(d),
  useUsageStats: (d: number) => mockUseUsageStats(d),
  useStorageStats: () => mockUseStorageStats(),
  useTriggerStorageScan: () => mockTriggerStorageScan,
  useDatabaseStats: () => mockUseDatabaseStats(),
  useSlowTransactions: () => mockUseSlowTransactions(),
  useHostMetrics: () => mockUseHostMetrics(),
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
  mockUseSlowTransactions.mockReturnValue({
    data: create(GetSlowTransactionsResponseSchema, {
      configured: true,
      current: [],
      trending: []
    }),
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
  mockUseLogs.mockReturnValue({
    data: create(GetLogsResponseSchema, { entries: [] }),
    isLoading: false
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
    expect(screen.getAllByText('CPU').length).toBeGreaterThan(0)
    expect(screen.getAllByText('12.3%').length).toBeGreaterThan(0)
  })

  it('refetches job/usage stats when the window changes', () => {
    render(<ObservabilityClient />)
    expect(mockUseJobStats).toHaveBeenCalledWith(30)

    fireEvent.change(screen.getByLabelText('Time window'), { target: { value: '7' } })
    expect(mockUseJobStats).toHaveBeenCalledWith(7)
    expect(mockUseUsageStats).toHaveBeenCalledWith(7)
  })

  it('links to the issues and monitoring settings pages', () => {
    render(<ObservabilityClient />)
    expect(screen.getByRole('link', { name: 'Issues' })).toHaveAttribute(
      'href',
      '/monitoring/issues'
    )
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute(
      'href',
      '/monitoring/settings'
    )
  })

  it('revalidates every data source when Refresh is clicked', async () => {
    render(<ObservabilityClient />)

    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))

    expect(screen.getByRole('button', { name: 'Refreshing…' })).toBeDisabled()
    // storageStats is refreshed via triggerStorageScan (a live R2 rescan)
    // instead of a plain mutate(), so mockMutate covers the other 5 sources.
    expect(mockMutate).toHaveBeenCalledTimes(5)
    expect(mockTriggerStorageScan).toHaveBeenCalledTimes(1)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh' })).not.toBeDisabled())
  })
})
