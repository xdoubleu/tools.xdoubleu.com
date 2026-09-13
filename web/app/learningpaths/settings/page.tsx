import LearningPathsSettingsClient from '@/components/learningpaths/LearningPathsSettingsClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { TodoistService } from '@/lib/gen/learningpaths/v1/learningpaths_pb'

export default async function LearningPathsSettingsPage() {
  const client = await createServerClient(TodoistService)
  const status = await fetchOrNull(() => client.getTodoistConnectionStatus({}))

  return (
    <SWRFallback fallback={status ? { [swrKeys.todoistConnection]: status } : {}}>
      <LearningPathsSettingsClient />
    </SWRFallback>
  )
}
