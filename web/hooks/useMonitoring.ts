import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  ObservabilityService,
  ProviderConfigSchema
} from '@/lib/gen/observability/v1/observability_pb'
import type {
  GetJobStatsResponse,
  GetUsageStatsResponse,
  GetStorageStatsResponse,
  GetDatabaseStatsResponse,
  GetGithubIssuesResponse,
  GetSentryIssuesResponse,
  GetDeployStatusResponse,
  ListOAuthConnectionsResponse,
  GetProviderOptionsResponse
} from '@/lib/gen/observability/v1/observability_pb'
import { swrKeys } from '@/lib/swrKeys'

export type ProviderConfigInput = MessageInitShape<typeof ProviderConfigSchema>

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

// PROVIDER_DATA_KEYS maps a provider to the SWR key holding the data it
// unlocks, so useSetProviderConfig can flip that card to "configured"
// immediately instead of waiting for its own poll/revalidation.
const PROVIDER_DATA_KEYS: Record<string, string> = {
  github: swrKeys.monitoringGithubIssues,
  sentry: swrKeys.monitoringSentryIssues,
  digitalocean: swrKeys.monitoringDeployStatus
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

export function useSetProviderConfig() {
  const client = useMemo(() => createServiceClient(ObservabilityService), [])
  return useCallback(
    async (provider: string, config: ProviderConfigInput) => {
      await client.setProviderConfig({ provider, config })
      await mutate(swrKeys.monitoringOAuthConnections)
      const dataKey = PROVIDER_DATA_KEYS[provider]
      if (dataKey) await mutate(dataKey)
    },
    [client]
  )
}
