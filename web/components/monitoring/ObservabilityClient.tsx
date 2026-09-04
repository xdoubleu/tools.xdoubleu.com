'use client'

import Link from 'next/link'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { Select } from '@/components/ui/select'
import {
  useJobStats,
  useStorageStats,
  useTriggerStorageScan,
  useDatabaseStats,
  useDatabaseSizeHistory,
  useSlowTransactions,
  useTransactionLatencyHistory,
  useHostMetrics,
  useAlertStates
} from '@/hooks/useMonitoring'
import { Collapsible } from '@/components/ui/collapsible'
import StorageCard from './StorageCard'
import DatabaseCard from './DatabaseCard'
import JobsCard from './JobsCard'
import SlowTransactionsCard from './SlowTransactionsCard'
import TransactionLatencyHistoryCard from './TransactionLatencyHistoryCard'
import HostMetricsCard from './HostMetricsCard'
import LogsCard from './LogsCard'

const WINDOW_OPTIONS = [7, 30, 90]

export default function ObservabilityClient() {
  const [windowDays, setWindowDays] = useState(30)
  const [isRefreshing, setIsRefreshing] = useState(false)

  const jobStats = useJobStats(windowDays)
  const storageStats = useStorageStats()
  const triggerStorageScan = useTriggerStorageScan()
  const databaseStats = useDatabaseStats(windowDays)
  const databaseSizeHistory = useDatabaseSizeHistory(windowDays)
  const slowTransactions = useSlowTransactions()
  const transactionLatencyHistory = useTransactionLatencyHistory(windowDays)
  const hostMetrics = useHostMetrics()
  const alertStates = useAlertStates()

  const refreshAll = async () => {
    setIsRefreshing(true)
    await Promise.all([
      jobStats.mutate(),
      triggerStorageScan(),
      databaseStats.mutate(),
      databaseSizeHistory.mutate(),
      slowTransactions.mutate(),
      transactionLatencyHistory.mutate(),
      hostMetrics.mutate(),
      alertStates.mutate()
    ])
    setIsRefreshing(false)
  }

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

      <div className="mt-6 space-y-4">
        <Collapsible title="Storage">
          <StorageCard data={storageStats.data} />
        </Collapsible>
        <Collapsible title="Database">
          <DatabaseCard data={databaseStats.data} history={databaseSizeHistory.data} />
        </Collapsible>
        <Collapsible title="Jobs">
          <JobsCard data={jobStats.data} />
        </Collapsible>
        <Collapsible title="Slow Spans">
          <SlowTransactionsCard data={slowTransactions.data} alertStates={alertStates.data} />
        </Collapsible>
        <Collapsible title="Span Latency History">
          <TransactionLatencyHistoryCard data={transactionLatencyHistory.data} />
        </Collapsible>
        <Collapsible title="Host Metrics">
          <HostMetricsCard data={hostMetrics.data} />
        </Collapsible>
        <Collapsible title="Logs">
          <LogsCard />
        </Collapsible>
      </div>
    </PageContainer>
  )
}
