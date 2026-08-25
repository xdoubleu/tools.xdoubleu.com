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
  useFailingPullRequests,
  useWorkflowRuns,
  useSecurityAlerts,
  useSentryIssues,
  useSlowTransactions,
  useHostMetrics,
  useNotificationSettings
} from '@/hooks/useMonitoring'
import { formatBytes, formatCount } from '@/lib/observability'
import StatTiles from './StatTiles'
import StorageCard from './StorageCard'
import DatabaseCard from './DatabaseCard'
import JobsCard from './JobsCard'
import UsageCard from './UsageCard'
import FailingPullRequestsCard from './FailingPullRequestsCard'
import WorkflowRunsCard from './WorkflowRunsCard'
import SecurityAlertsCard from './SecurityAlertsCard'
import SentryCard from './SentryCard'
import SlowTransactionsCard from './SlowTransactionsCard'
import HostMetricsCard from './HostMetricsCard'
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
  const failingPullRequests = useFailingPullRequests()
  const workflowRuns = useWorkflowRuns()
  const securityAlerts = useSecurityAlerts()
  const sentryIssues = useSentryIssues()
  const slowTransactions = useSlowTransactions()
  const hostMetrics = useHostMetrics()
  const notificationSettings = useNotificationSettings()

  const refreshAll = async () => {
    setIsRefreshing(true)
    await Promise.all([
      jobStats.mutate(),
      usageStats.mutate(),
      triggerStorageScan(),
      databaseStats.mutate(),
      failingPullRequests.mutate(),
      workflowRuns.mutate(),
      securityAlerts.mutate(),
      sentryIssues.mutate(),
      slowTransactions.mutate(),
      hostMetrics.mutate(),
      notificationSettings.mutate()
    ])
    setIsRefreshing(false)
  }

  const latest = storageStats.data?.latest
  const failingJobs = (jobStats.data?.stats ?? []).filter((s) => Number(s.failedRuns) > 0).length

  const failingPRs = failingPullRequests.data
  const alerts = securityAlerts.data
  const sentry = sentryIssues.data
  const failingCount = failingPRs?.configured ? failingPRs.failingCount : 0
  const alertCount = alerts?.configured ? alerts.alertCount : 0
  const unresolvedErrors = sentry?.configured ? sentry.unresolvedCount : 0

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
      label: 'Failing PRs',
      value: failingPRs?.configured ? formatCount(failingCount) : '—',
      tone: failingCount > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Security alerts',
      value: alerts?.configured ? formatCount(alertCount) : '—',
      tone: alertCount > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Unresolved errors',
      value: sentry?.configured ? formatCount(unresolvedErrors) : '—',
      tone: unresolvedErrors > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'CPU',
      value: hostMetrics.data ? `${hostMetrics.data.cpuPercent.toFixed(1)}%` : '—'
    },
    {
      label: 'Memory',
      value: hostMetrics.data ? `${hostMetrics.data.memoryPercent.toFixed(1)}%` : '—'
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
        </div>
      </div>

      <StatTiles tiles={tiles} />

      <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <StorageCard data={storageStats.data} />
        <DatabaseCard data={databaseStats.data} />
        <JobsCard data={jobStats.data} />
        <UsageCard data={usageStats.data} />
        <FailingPullRequestsCard data={failingPullRequests.data} />
        <WorkflowRunsCard data={workflowRuns.data} />
        <SecurityAlertsCard data={securityAlerts.data} />
        <SentryCard
          data={sentryIssues.data}
          emailEnabled={
            notificationSettings.data?.settings.find((s) => s.sourceKey === 'sentry_issues')
              ?.enabled
          }
        />
        <SlowTransactionsCard data={slowTransactions.data} />
        <HostMetricsCard data={hostMetrics.data} />
        <LogsCard />
      </div>
    </PageContainer>
  )
}
