import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { TaskService } from '@/lib/gen/todos/v1/tasks_connect'
import type { ListTasksResponse } from '@/lib/gen/todos/v1/tasks_pb'

export function useTodos(
  queryParams?: {
    workspaceId?: string
    sectionId?: string
    status?: string
  }
) {
  const key = queryParams
    ? ['/todos', queryParams]
    : '/todos'
  const client = createServiceClient(TaskService)
  return useSWR<ListTasksResponse>(key, () =>
    client.listTasks({
      workspaceId: queryParams?.workspaceId ?? '',
      sectionId: queryParams?.sectionId ?? '',
      status: queryParams?.status ?? '',
    })
  )
}
