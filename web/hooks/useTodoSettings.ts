import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_connect'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'

export function useTodoSettings() {
  const client = createServiceClient(SettingsService)
  return useSWR<GetSettingsResponse>('/todos/settings', () =>
    client.getSettings({})
  )
}
