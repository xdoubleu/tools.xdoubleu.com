'use client'

import Link from 'next/link'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { Select } from '@/components/ui/select'
import {
  useJobStats,
  useUsageStats,
  useStorageStats,
  useTriggerStorageScan,
  useDatabaseStats,
  useSlowTransactions,
  useHostMetrics
} from '@/hooks/useMonitoring'
import { formatBytes, formatCount } from '@/lib/observability'
import StatTiles from './StatTiles'
import StorageCard from './StorageCard'
import DatabaseCard from './DatabaseCard'
import JobsCard from './JobsCard'
import UsageCard from './UsageCard'
import SlowTransactionsCard, { regressionDangerThreshold } from './SlowTransactionsCard'
import HostMetricsCard, { hostMetricTone } from './HostMetricsCard'
import LogsCard from './LogsCard'

const WINDOW_OPTIONS = [7, 30, 90]

export default function ObservabilityClient() {
  const [windowDays, setWindowDays] = useState(30)
  const [isRefreshing, setIsRefreshing] = useState(false)

  const jobStats = useJobStats(windowDays)
  const usageStats = useUsageStats(windowDays)
  const storageStats = useStorageStats()
  const triggerStorageScan = useTriggerStorageScan()
  const databaseStats = useDatabaseStats()
  const slowTransactions = useSlowTransactions()
  const hostMetrics = useHostMetrics()

  const refreshAll = async () => {
    setIsRefreshing(true)
    await Promise.all([
      jobStats.mutate(),
      usageStats.mutate(),
      triggerStorageScan(),
      databaseStats.mutate(),
      slowTransactions.mutate(),
      hostMetrics.mutate()
    ])
    setIsRefreshing(false)
  }

  const latest = storageStats.data?.latest
  const failingJobs = (jobStats.data?.stats ?? []).filter((s) => Number(s.failedRuns) > 0).length

  const hostMetricValues = hostMetrics.data
    ? [hostMetrics.data.cpuPercent, hostMetrics.data.memoryPercent, hostMetrics.data.diskPercent]
    : []
  const overThresholdMetrics = hostMetricValues.filter(
    (value) => hostMetricTone(value) !== 'default'
  ).length

  const regressingTransactions = (slowTransactions.data?.trending ?? []).filter(
    (t) => t.pctChange > regressionDangerThreshold
  ).length

  const tiles = [
    {
      label: 'R2 storage',
      value: latest ? formatBytes(latest.totalSizeBytes) : '—'
    },
    {
      label: 'Database',
      value: databaseStats.data ? formatBytes(databaseStats.data.totalSizeBytes) : '—'
    },
    {
      label: 'Orphaned',
      value: latest ? formatBytes(latest.orphanSizeBytes) : '—',
      tone: latest && Number(latest.orphanCount) > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Jobs failing',
      value: formatCount(failingJobs),
      tone: failingJobs > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'CPU',
      value: hostMetrics.data ? `${hostMetrics.data.cpuPercent.toFixed(1)}%` : '—'
    },
    {
      label: 'Memory',
      value: hostMetrics.data ? `${hostMetrics.data.memoryPercent.toFixed(1)}%` : '—'
    },
    {
      label: 'Over threshold',
      value: hostMetrics.data ? formatCount(overThresholdMetrics) : '—',
      tone: overThresholdMetrics > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Regressing',
      value: slowTransactions.data ? formatCount(regressingTransactions) : '—',
      tone: regressingTransactions > 0 ? ('danger' as const) : ('default' as const)
    }
  ]

  return (
    <PageContainer className="p-6">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-bold">Observability</h1>
        <div className="flex items-center gap-3">
          <Button variant="secondary" asChild>
            <Link href="/monitoring/settings">Settings</Link>
          </Button>
          <Button variant="secondary" onClick={refreshAll} disabled={isRefreshing}>
            {isRefreshing ? 'Refreshing…' : 'Refresh'}
          </Button>
          <Select
            value={String(windowDays)}
            onChange={(e) => setWindowDays(Number(e.target.value))}
            className="h-9 w-auto"
            aria-label="Time window"
          >
            {WINDOW_OPTIONS.map((d) => (
              <option key={d} value={d}>
                Last {d} days
              </option>
            ))}
          </Select>
          <Link
            href="/monitoring"
            className="text-sm text-accent underline-offset-4 hover:underline"
          >
            Back to monitoring
          </Link>
        </div>
      </div>

      <StatTiles tiles={tiles} />

      <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <StorageCard data={storageStats.data} />
        <DatabaseCard data={databaseStats.data} />
        <JobsCard data={jobStats.data} />
        <UsageCard data={usageStats.data} />
        <SlowTransactionsCard data={slowTransactions.data} />
        <HostMetricsCard data={hostMetrics.data} />
        <LogsCard />
      </div>
    </PageContainer>
  )
}
