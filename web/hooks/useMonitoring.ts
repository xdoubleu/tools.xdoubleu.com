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
  GetNotificationSettingsResponse,
  GetAutomatedActionsResponse
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

// Fetched on demand when the picker opens, not via SWR.
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

// useNotificationSettings reads which sources may email an admin.
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

// useAutomatedActions reads recent global.automated_actions runs (the RPC's
// default window is 30 days).
export function useAutomatedActions() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetAutomatedActionsResponse, Error>(swrKeys.monitoringAutomatedActions, () =>
    client.getAutomatedActions({})
  )
}
