import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  ObservabilityService,
  ProviderConfigSchema,
  SecurityAlertType
} from '@/lib/gen/observability/v1/observability_pb'
import type {
  GetJobStatsResponse,
  GetStorageStatsResponse,
  GetDatabaseStatsResponse,
  GetDatabaseSizeHistoryResponse,
  GetFailingPullRequestsResponse,
  GetWorkflowRunsResponse,
  GetSecurityAlertsResponse,
  GetSentryIssuesResponse,
  GetSlowTransactionsResponse,
  GetTransactionLatencyHistoryResponse,
  GetHostMetricsResponse,
  GetLogsResponse,
  ListOAuthConnectionsResponse,
  GetProviderOptionsResponse,
  GetNotificationSettingsResponse,
  GetAlertStatesResponse
} from '@/lib/gen/observability/v1/observability_pb'
import { swrKeys } from '@/lib/swrKeys'

export type ProviderConfigInput = MessageInitShape<typeof ProviderConfigSchema>

export function useJobStats(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetJobStatsResponse, Error>(swrKeys.monitoringJobStats(windowDays), () =>
    client.getJobStats({ windowDays })
  )
}

export function useStorageStats() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetStorageStatsResponse, Error>(swrKeys.monitoringStorageStats, () =>
    client.getStorageStats({})
  )
}

// useTriggerStorageScan runs a live R2 rescan (instead of just re-reading the
// last daily-job snapshot), then revalidates storage stats so the fresh scan
// shows up.
export function useTriggerStorageScan() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(async () => {
    await client.triggerStorageScan({})
    await mutate(swrKeys.monitoringStorageStats)
  }, [client])
}

export function useDatabaseStats(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetDatabaseStatsResponse, Error>(swrKeys.monitoringDatabaseStats(windowDays), () =>
    client.getDatabaseStats({ windowDays })
  )
}

export function useDatabaseSizeHistory(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetDatabaseSizeHistoryResponse, Error>(
    swrKeys.monitoringDatabaseSizeHistory(windowDays),
    () => client.getDatabaseSizeHistory({ windowDays })
  )
}

export function useFailingPullRequests() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetFailingPullRequestsResponse, Error>(swrKeys.monitoringFailingPullRequests, () =>
    client.getFailingPullRequests({})
  )
}

export function useWorkflowRuns() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetWorkflowRunsResponse, Error>(swrKeys.monitoringWorkflowRuns, () =>
    client.getWorkflowRuns({})
  )
}

export function useSecurityAlerts() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetSecurityAlertsResponse, Error>(swrKeys.monitoringSecurityAlerts, () =>
    client.getSecurityAlerts({})
  )
}

// useDismissSecurityAlert dismisses/resolves one open GitHub Dependabot,
// code-scanning, or secret-scanning alert. Valid reasons differ per
// alertType — the caller (the reason picker in SecurityAlertsCard) is
// responsible for offering only the values GitHub's API accepts for it.
export function useDismissSecurityAlert() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(
    async (alertType: SecurityAlertType, alertNumber: bigint, reason: string) => {
      await client.dismissSecurityAlert({ alertType, alertNumber, reason })
      await mutate(swrKeys.monitoringSecurityAlerts)
    },
    [client]
  )
}

// useAlertStates reads the current breach state of every threshold alert
// rule (jobs.ThresholdAlertJob, issue #1283). current_value/threshold are
// refreshed on every evaluation, not just on a breach/recovery transition,
// so a non-breaching rule still reports a meaningful current value.
export function useAlertStates() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetAlertStatesResponse, Error>(swrKeys.monitoringAlertStates, () =>
    client.getAlertStates({})
  )
}

export function useSentryIssues() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetSentryIssuesResponse, Error>(swrKeys.monitoringSentryIssues, () =>
    client.getSentryIssues({})
  )
}

export function useResolveSentryIssue() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(
    async (issueId: string) => {
      await client.resolveSentryIssue({ issueId })
      await mutate(swrKeys.monitoringSentryIssues)
    },
    [client]
  )
}

export function useSlowTransactions() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetSlowTransactionsResponse, Error>(swrKeys.monitoringSlowTransactions, () =>
    client.getSlowTransactions({})
  )
}

// useTransactionLatencyHistory reads every stored (project, transaction)
// daily p95 series over the window, flat — the chart pivots and selects
// which series to plot.
export function useTransactionLatencyHistory(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetTransactionLatencyHistoryResponse, Error>(
    swrKeys.monitoringTransactionLatencyHistory(windowDays),
    () => client.getTransactionLatencyHistory({ windowDays })
  )
}

// useHostMetrics polls the host's CPU/memory/disk usage, scraped from
// node_exporter (issue #1040). since bounds how far back the history series
// go; empty defaults to the server's own retention window.
export function useHostMetrics(since = '') {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetHostMetricsResponse, Error>(swrKeys.monitoringHostMetrics, () =>
    client.getHostMetrics({ since })
  )
}

// useLogs reads recent application logs forwarded from both api and web
// (global.log_entries, issue #1040). source/minLevel empty means "any".
export function useLogs(source = '', minLevel = '', since = '') {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetLogsResponse, Error>(swrKeys.monitoringLogs(source, minLevel), () =>
    client.getLogs({ source, minLevel, since })
  )
}

export function useOAuthConnections() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<ListOAuthConnectionsResponse, Error>(swrKeys.monitoringOAuthConnections, () =>
    client.listOAuthConnections({})
  )
}

export function useDisconnectOAuthConnection() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(
    async (provider: string) => {
      await client.disconnectOAuthConnection({ provider })
      await mutate(swrKeys.monitoringOAuthConnections)
    },
    [client]
  )
}

// PROVIDER_DATA_KEYS maps a provider to the SWR key(s) holding the data it
// unlocks, so useSetProviderConfig can flip those cards to "configured"
// immediately instead of waiting for their own poll/revalidation.
const PROVIDER_DATA_KEYS: Record<string, string[]> = {
  github: [swrKeys.monitoringFailingPullRequests, swrKeys.monitoringSecurityAlerts],
  sentry: [swrKeys.monitoringSentryIssues]
}

// useProviderOptions is fetched on demand (when the config picker dialog
// opens), not via SWR — matching useDisconnectOAuthConnection's callback
// pattern above.
export function useProviderOptions() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(
    (provider: string, sentryOrg?: string): Promise<GetProviderOptionsResponse> =>
      client.getProviderOptions({ provider, sentryOrg: sentryOrg ?? '' }),
    [client]
  )
}

// useNotificationSettings reads which sources (Sentry issues, failing
// dependency PRs, unhealthy feeds) are currently allowed to email an admin
// (issue #1214).
export function useNotificationSettings() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetNotificationSettingsResponse, Error>(
    swrKeys.monitoringNotificationSettings,
    () => client.getNotificationSettings({})
  )
}

export function useUpdateNotificationSettings() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(
    async (sourceKey: string, enabled: boolean) => {
      await client.updateNotificationSettings({ sourceKey, enabled })
      await mutate(swrKeys.monitoringNotificationSettings)
    },
    [client]
  )
}

export function useSetProviderConfig() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(
    async (provider: string, config: ProviderConfigInput) => {
      await client.setProviderConfig({ provider, config })
      await mutate(swrKeys.monitoringOAuthConnections)
      const dataKeys = PROVIDER_DATA_KEYS[provider] ?? []
      await Promise.all(dataKeys.map((key) => mutate(key)))
    },
    [client]
  )
}
