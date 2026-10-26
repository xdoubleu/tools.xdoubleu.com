// Which sources may email an admin: set from monitoring and from each app's
// settings page.
import { useCallback, useMemo } from 'react'
import useSWR, { mutate } from 'swr'
import { createServiceClient } from '@/lib/client'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import type { GetNotificationSettingsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { swrKeys } from '@/lib/swrKeys'

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
