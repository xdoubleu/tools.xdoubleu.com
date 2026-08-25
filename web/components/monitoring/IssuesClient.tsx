'use client'

import Link from 'next/link'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import {
  useFailingPullRequests,
  useWorkflowRuns,
  useSecurityAlerts,
  useSentryIssues,
  useNotificationSettings
} from '@/hooks/useMonitoring'
import { formatCount } from '@/lib/observability'
import StatTiles from './StatTiles'
import FailingPullRequestsCard from './FailingPullRequestsCard'
import WorkflowRunsCard from './WorkflowRunsCard'
import SecurityAlertsCard from './SecurityAlertsCard'
import SentryCard from './SentryCard'

export default function IssuesClient() {
  const [isRefreshing, setIsRefreshing] = useState(false)

  const failingPullRequests = useFailingPullRequests()
  const workflowRuns = useWorkflowRuns()
  const securityAlerts = useSecurityAlerts()
  const sentryIssues = useSentryIssues()
  const notificationSettings = useNotificationSettings()

  const refreshAll = async () => {
    setIsRefreshing(true)
    await Promise.all([
      failingPullRequests.mutate(),
      workflowRuns.mutate(),
      securityAlerts.mutate(),
      sentryIssues.mutate()
    ])
    setIsRefreshing(false)
  }

  const failingPRs = failingPullRequests.data
  const alerts = securityAlerts.data
  const sentry = sentryIssues.data
  const failingCount = failingPRs?.configured ? failingPRs.failingCount : 0
  const alertCount = alerts?.configured ? alerts.alertCount : 0
  const unresolvedErrors = sentry?.configured ? sentry.unresolvedCount : 0

  const mainFailingRuns = (workflowRuns.data?.runs ?? []).filter(
    (run) => run.event === 'push' && run.branch === 'main' && run.conclusion === 'failure'
  )

  const tiles = [
    {
      label: 'Security alerts',
      value: alerts?.configured ? formatCount(alertCount) : '—',
      tone: alertCount > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Failing dependency PRs',
      value: failingPRs?.configured ? formatCount(failingCount) : '—',
      tone: failingCount > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Failing runs on main',
      value: workflowRuns.data?.configured ? formatCount(mainFailingRuns.length) : '—',
      tone: mainFailingRuns.length > 0 ? ('danger' as const) : ('default' as const)
    },
    {
      label: 'Unresolved errors',
      value: sentry?.configured ? formatCount(unresolvedErrors) : '—',
      tone: unresolvedErrors > 0 ? ('danger' as const) : ('default' as const)
    }
  ]

  return (
    <PageContainer className="p-6">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-bold">Issues</h1>
        <div className="flex items-center gap-3">
          <Button variant="secondary" asChild>
            <Link href="/monitoring/observability">Observability</Link>
          </Button>
          <Button variant="secondary" asChild>
            <Link href="/monitoring/settings">Settings</Link>
          </Button>
          <Button variant="secondary" onClick={refreshAll} disabled={isRefreshing}>
            {isRefreshing ? 'Refreshing…' : 'Refresh'}
          </Button>
        </div>
      </div>

      <StatTiles tiles={tiles} />

      <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <SecurityAlertsCard data={securityAlerts.data} />
        <FailingPullRequestsCard data={failingPullRequests.data} />
        <WorkflowRunsCard
          data={
            workflowRuns.data ? { ...workflowRuns.data, runs: mainFailingRuns } : workflowRuns.data
          }
          title="Failing runs on main"
          description="GitHub Actions runs on main with a failing conclusion."
        />
        <SentryCard
          data={sentryIssues.data}
          emailEnabled={
            notificationSettings.data?.settings.find((s) => s.sourceKey === 'sentry_issues')
              ?.enabled
          }
        />
      </div>
    </PageContainer>
  )
}
