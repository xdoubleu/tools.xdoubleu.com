'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { Select } from '@/components/ui/select'
import {
  useJobStats,
  useUsageStats,
  useStorageStats,
  useDatabaseStats,
  useGithubIssues,
  useSentryIssues,
  useDeployStatus,
  useOAuthConnections
} from '@/hooks/useMonitoring'
import { formatBytes, formatCount } from '@/lib/observability'
import StatTiles from './StatTiles'
import StorageCard from './StorageCard'
import DatabaseCard from './DatabaseCard'
import JobsCard from './JobsCard'
import UsageCard from './UsageCard'
import GithubIssuesCard from './GithubIssuesCard'
import SentryCard from './SentryCard'
import DeployCard from './DeployCard'
import OAuthConnectionsCard from './OAuthConnectionsCard'

const WINDOW_OPTIONS = [7, 30, 90]

export default function ObservabilityClient() {
  const [windowDays, setWindowDays] = useState(30)
  const [isRefreshing, setIsRefreshing] = useState(false)

  const jobStats = useJobStats(windowDays)
  const usageStats = useUsageStats(windowDays)
  const storageStats = useStorageStats()
  const databaseStats = useDatabaseStats()
  const githubIssues = useGithubIssues()
  const sentryIssues = useSentryIssues()
  const deployStatus = useDeployStatus()
  const oauthConnections = useOAuthConnections()

  const refreshAll = async () => {
    setIsRefreshing(true)
    await Promise.all([
      jobStats.mutate(),
      usageStats.mutate(),
      storageStats.mutate(),
      databaseStats.mutate(),
      githubIssues.mutate(),
      sentryIssues.mutate(),
      deployStatus.mutate(),
      oauthConnections.mutate()
    ])
    setIsRefreshing(false)
  }

  const latest = storageStats.data?.latest
  const failingJobs = (jobStats.data?.stats ?? []).filter((s) => Number(s.failedRuns) > 0).length

  const github = githubIssues.data
  const sentry = sentryIssues.data
  const deploy = deployStatus.data
  const openIssues = github?.configured ? github.openCount : 0
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
      label: 'Open issues',
      value: github?.configured ? formatCount(openIssues) : '—',
      tone: openIssues > 0 ? ('danger' as const) : ('default' as const)
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

      <StatTiles tiles={tiles} />

      <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <StorageCard data={storageStats.data} />
        <DatabaseCard data={databaseStats.data} />
        <JobsCard data={jobStats.data} />
        <UsageCard data={usageStats.data} />
        <GithubIssuesCard data={githubIssues.data} />
        <SentryCard data={sentryIssues.data} />
        <DeployCard data={deployStatus.data} />
        <OAuthConnectionsCard data={oauthConnections.data} />
      </div>
    </PageContainer>
  )
}
