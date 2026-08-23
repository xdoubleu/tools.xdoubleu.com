'use client'

import { useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
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
  useDeployStatus,
  useOAuthConnections
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
import DeployCard from './DeployCard'
import OAuthConnectionsCard from './OAuthConnectionsCard'

const WINDOW_OPTIONS = [7, 30, 90]

export default function ObservabilityClient() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [windowDays, setWindowDays] = useState(30)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [oauthMessage, setOAuthMessage] = useState<{
    tone: 'success' | 'danger'
    text: string
  } | null>(null)

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
  const deployStatus = useDeployStatus()
  const oauthConnections = useOAuthConnections()

  useEffect(() => {
    const connected = searchParams.get('oauth_connected')
    const errored = searchParams.get('oauth_error')
    if (!connected && !errored) return

    if (connected) {
      setOAuthMessage({ tone: 'success', text: `Connected ${connected}.` })
      void oauthConnections.mutate()
    } else if (errored) {
      setOAuthMessage({
        tone: 'danger',
        text: `Failed to connect ${errored}. Check the server logs for details.`
      })
    }

    const params = new URLSearchParams(searchParams)
    params.delete('oauth_connected')
    params.delete('oauth_error')
    router.replace(params.size > 0 ? `/monitoring?${params}` : '/monitoring')
    // oauthConnections/router deliberately excluded: this should run once per
    // incoming URL, not on every SWR/router identity change.
  }, [searchParams])

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
      deployStatus.mutate(),
      oauthConnections.mutate()
    ])
    setIsRefreshing(false)
  }

  const latest = storageStats.data?.latest
  const failingJobs = (jobStats.data?.stats ?? []).filter((s) => Number(s.failedRuns) > 0).length

  const failingPRs = failingPullRequests.data
  const alerts = securityAlerts.data
  const sentry = sentryIssues.data
  const deploy = deployStatus.data
  const failingCount = failingPRs?.configured ? failingPRs.failingCount : 0
  const alertCount = alerts?.configured ? alerts.alertCount : 0
  const unresolvedErrors = sentry?.configured ? sentry.unresolvedCount : 0
  const deployPhase = deploy?.configured ? deploy.phase : ''

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
      label: 'Deploy',
      value: deployPhase || '—',
      tone:
        deployPhase === 'ERROR' || deployPhase === 'CANCELED'
          ? ('danger' as const)
          : ('default' as const)
    }
  ]

  return (
    <PageContainer className="p-6">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-bold">Observability</h1>
        <div className="flex items-center gap-3">
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

      {oauthMessage && (
        <p
          className={`mb-6 text-sm ${oauthMessage.tone === 'success' ? 'text-success' : 'text-danger'}`}
        >
          {oauthMessage.text}
        </p>
      )}

      <StatTiles tiles={tiles} />

      <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <StorageCard data={storageStats.data} />
        <DatabaseCard data={databaseStats.data} />
        <JobsCard data={jobStats.data} />
        <UsageCard data={usageStats.data} />
        <FailingPullRequestsCard data={failingPullRequests.data} />
        <WorkflowRunsCard data={workflowRuns.data} />
        <SecurityAlertsCard data={securityAlerts.data} />
        <SentryCard data={sentryIssues.data} />
        <SlowTransactionsCard data={slowTransactions.data} />
        <DeployCard data={deployStatus.data} />
        <OAuthConnectionsCard data={oauthConnections.data} />
      </div>
    </PageContainer>
  )
}
