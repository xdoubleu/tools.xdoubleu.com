import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'
import type { GetSettingsResponse } from '@/lib/gen/todos/v1/settings_pb'
import { swrKeys } from '@/lib/swrKeys'

export function useTodoSettings() {
  const client = createServiceClient(SettingsService)
  return useSWR<GetSettingsResponse>(swrKeys.todoSettings, () => client.getSettings({}))
}
