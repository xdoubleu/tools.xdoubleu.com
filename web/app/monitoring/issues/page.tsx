import IssuesClient from '@/components/monitoring/IssuesClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'

export default async function MonitoringIssuesPage() {
  const client = await createServerClient(ObservabilityService)

  const [failingPullRequests, workflowRuns, securityAlerts, sentryIssues] = await Promise.all([
    fetchOrNull(() => client.getFailingPullRequests({})),
    fetchOrNull(() => client.getWorkflowRuns({})),
    fetchOrNull(() => client.getSecurityAlerts({})),
    fetchOrNull(() => client.getSentryIssues({}))
  ])

  const fallback: Record<string, unknown> = {}
  if (failingPullRequests) fallback[swrKeys.monitoringFailingPullRequests] = failingPullRequests
  if (workflowRuns) fallback[swrKeys.monitoringWorkflowRuns] = workflowRuns
  if (securityAlerts) fallback[swrKeys.monitoringSecurityAlerts] = securityAlerts
  if (sentryIssues) fallback[swrKeys.monitoringSentryIssues] = sentryIssues

  return (
    <SWRFallback fallback={fallback}>
      <IssuesClient />
    </SWRFallback>
  )
}
