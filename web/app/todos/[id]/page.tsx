import TaskClient from './TaskClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { TaskService } from '@/lib/gen/todos/v1/tasks_pb'

export default async function TaskPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const client = await createServerClient(TaskService)
  const task = await fetchOrNull(() => client.getTask({ id }))

  return (
    <SWRFallback fallback={task ? { [swrKeys.todoTask(id)]: task } : {}}>
      <TaskClient id={id} />
    </SWRFallback>
  )
}
