import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { TaskService } from '@/lib/gen/todos/v1/tasks_pb'
import type { ListTasksResponse } from '@/lib/gen/todos/v1/tasks_pb'
import { swrKeys } from '@/lib/swrKeys'

// ponytail: the open-tasks list has no "load more" UI (TodosPageClient's
// keyboard nav/search/mutate wiring makes that a bigger change than this
// warrants), so request the backend's max page size rather than its default
// — bounds the query without silently hiding tasks for realistic task counts.
// Upgrade path: add a load-more affordance if someone's open list exceeds 200.
const OPEN_TASKS_LIMIT = 200

export function useTodos(queryParams?: {
  workspaceId?: string
  sectionId?: string
  status?: string
}) {
  const key = queryParams ? swrKeys.todosFiltered(queryParams) : swrKeys.todos
  const client = createServiceClient(TaskService)
  return useSWR<ListTasksResponse>(key, () =>
    client.listTasks({
      workspaceId: queryParams?.workspaceId ?? '',
      sectionId: queryParams?.sectionId ?? '',
      status: queryParams?.status ?? '',
      limit: OPEN_TASKS_LIMIT
    })
  )
}
