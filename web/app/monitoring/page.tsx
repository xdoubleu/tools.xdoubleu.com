import IssuesClient from '@/components/monitoring/IssuesClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'

export default async function MonitoringPage() {
  const client = await createServerClient(ObservabilityService)

  const [failingPullRequests, workflowRuns, securityAlerts, sentryIssues, alertStates] =
    await Promise.all([
      fetchOrNull(() => client.getFailingPullRequests({})),
      fetchOrNull(() => client.getWorkflowRuns({})),
      fetchOrNull(() => client.getSecurityAlerts({})),
      fetchOrNull(() => client.getSentryIssues({})),
      fetchOrNull(() => client.getAlertStates({}))
    ])

  const fallback: Record<string, unknown> = {}
  if (failingPullRequests) fallback[swrKeys.monitoringFailingPullRequests] = failingPullRequests
  if (workflowRuns) fallback[swrKeys.monitoringWorkflowRuns] = workflowRuns
  if (securityAlerts) fallback[swrKeys.monitoringSecurityAlerts] = securityAlerts
  if (sentryIssues) fallback[swrKeys.monitoringSentryIssues] = sentryIssues
  if (alertStates) fallback[swrKeys.monitoringAlertStates] = alertStates

  return (
    <SWRFallback fallback={fallback}>
      <IssuesClient />
    </SWRFallback>
  )
}
