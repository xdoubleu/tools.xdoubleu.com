import TodosPageClient from '@/components/todos/TodosPageClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { TaskService } from '@/lib/gen/todos/v1/tasks_pb'
import { SettingsService } from '@/lib/gen/todos/v1/settings_pb'

export default async function TodosPage({
  searchParams
}: {
  searchParams: Promise<{ w?: string }>
}) {
  const { w } = await searchParams
  const [tasksClient, settingsClient] = await Promise.all([
    createServerClient(TaskService),
    createServerClient(SettingsService)
  ])
  const [tasks, settings] = await Promise.all([
    fetchOrNull(() => tasksClient.listTasks({ workspaceId: w ?? '', sectionId: '', status: '' })),
    fetchOrNull(() => settingsClient.getSettings({}))
  ])

  // The keyed entry must mirror TodosPageClient's initial useTodos() call
  // exactly: 'active' tab and no section selected on first render.
  return (
    <SWRFallback
      fallback={settings ? { [swrKeys.todoSettings]: settings } : {}}
      keyed={
        tasks
          ? [
              [
                swrKeys.todosFiltered({
                  workspaceId: w ?? undefined,
                  sectionId: undefined,
                  status: undefined
                }),
                tasks
              ]
            ]
          : []
      }
    >
      <TodosPageClient />
    </SWRFallback>
  )
}
