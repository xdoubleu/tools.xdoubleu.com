import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { createServiceClient } from '@/lib/client'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import type {
  GetJobStatsResponse,
  GetUsageStatsResponse,
  GetStorageStatsResponse,
  GetDatabaseStatsResponse,
  GetGithubIssuesResponse,
  GetSentryIssuesResponse,
  GetDeployStatusResponse,
  ListOAuthConnectionsResponse
} from '@/lib/gen/observability/v1/observability_pb'
import { swrKeys } from '@/lib/swrKeys'

export function useJobStats(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetJobStatsResponse, Error>(swrKeys.monitoringJobStats(windowDays), () =>
    client.getJobStats({ windowDays })
  )
}

export function useUsageStats(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetUsageStatsResponse, Error>(swrKeys.monitoringUsageStats(windowDays), () =>
    client.getUsageStats({ windowDays })
  )
}

export function useStorageStats() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetStorageStatsResponse, Error>(swrKeys.monitoringStorageStats, () =>
    client.getStorageStats({})
  )
}

export function useDatabaseStats() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetDatabaseStatsResponse, Error>(swrKeys.monitoringDatabaseStats, () =>
    client.getDatabaseStats({})
  )
}

export function useGithubIssues() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetGithubIssuesResponse, Error>(swrKeys.monitoringGithubIssues, () =>
    client.getGithubIssues({})
  )
}

export function useSentryIssues() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetSentryIssuesResponse, Error>(swrKeys.monitoringSentryIssues, () =>
    client.getSentryIssues({})
  )
}

export function useDeployStatus() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetDeployStatusResponse, Error>(swrKeys.monitoringDeployStatus, () =>
    client.getDeployStatus({})
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
