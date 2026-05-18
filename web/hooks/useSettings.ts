import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { SettingsService } from '@/lib/gen/settings/v1/settings_connect'
import type {
  GetSettingsResponse,
  Integrations,
} from '@/lib/gen/settings/v1/settings_pb'

export function useSettings() {
  const client = createServiceClient(SettingsService)
  return useSWR<GetSettingsResponse, Error>('/settings', () =>
    client.getSettings({})
  )
}

export function useSaveSettings() {
  const client = createServiceClient(SettingsService)
  return (integrations: Integrations) =>
    client.saveSettings({ integrations })
}
