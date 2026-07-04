import TodoSettingsClient from '@/components/todos/TodoSettingsClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'

export default async function TodoSettingsPage() {
  const client = await createServerClient(SettingsService)
  const settings = await fetchOrNull(() => client.getSettings({}))

  return (
    <SWRFallback fallback={settings ? { [swrKeys.todoSettings]: settings } : {}}>
      <TodoSettingsClient />
    </SWRFallback>
  )
}
