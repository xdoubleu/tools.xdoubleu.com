import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  ObservabilityService,
  ProviderConfigSchema
} from '@/lib/gen/observability/v1/observability_pb'
import type {
  ListOAuthConnectionsResponse,
  GetProviderOptionsResponse,
  GetNotificationSettingsResponse
} from '@/lib/gen/observability/v1/observability_pb'
import { swrKeys } from '@/lib/swrKeys'

export type ProviderConfigInput = MessageInitShape<typeof ProviderConfigSchema>

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
    },
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
