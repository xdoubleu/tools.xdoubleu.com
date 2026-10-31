import useSWR, { useSWRConfig } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { RepoService, TaskService } from '@/lib/gen/factory/v1/factory_pb'
import type {
  GetTaskResponse,
  Harness,
  ListReposResponse,
  ListTasksResponse,
  TaskPriority,
  TaskStatus
} from '@/lib/gen/factory/v1/factory_pb'

export function useFactoryTasks() {
  const client = createServiceClient(TaskService)
  return useSWR<ListTasksResponse, Error>(swrKeys.factoryTasks, () => client.listTasks({}))
}

export function useFactoryRepos() {
  const client = createServiceClient(RepoService)
  return useSWR<ListReposResponse, Error>(swrKeys.factoryRepos, () => client.listRepos({}))
}

/** One task with its comments; no fetch without an id. */
export function useFactoryTask(id: string | null) {
  const client = createServiceClient(TaskService)
  return useSWR<GetTaskResponse, Error>(id ? swrKeys.factoryTask(id) : null, () =>
    client.getTask({ id: id ?? '' })
  )
}

interface TaskFields {
  title: string
  body: string
  priority: TaskPriority
  labels: string[]
}

export interface RepoFields {
  owner: string
  name: string
  defaultBranch: string
  defaultHarness: Harness
  setupCommands: string
  agentInstructions: string
  timeLimitMinutes: number
}

const isFactoryKey = (key: unknown) => {
  const path: unknown = Array.isArray(key) ? key[0] : key
  return typeof path === 'string' && path.startsWith('/factory/')
}

/** Board writes; each revalidates the board and every open task. */
export function useFactoryActions() {
  const client = createServiceClient(TaskService)
  const repoClient = createServiceClient(RepoService)
  const { mutate } = useSWRConfig()
  const refresh = () => mutate(isFactoryKey)

  return {
    create: async (fields: TaskFields & { repoId: string }) => {
      const { task } = await client.createTask(fields)
      await refresh()
      return task
    },
    update: async (id: string, fields: TaskFields) => {
      await client.updateTask({ id, ...fields })
      await refresh()
    },
    move: async (id: string, status: TaskStatus) => {
      await client.moveTask({ id, status })
      await refresh()
    },
    comment: async (taskId: string, body: string) => {
      await client.addComment({ taskId, body })
      await refresh()
    },
    createRepo: async (repo: RepoFields) => {
      await repoClient.createRepo({ repo })
      await refresh()
    },
    updateRepo: async (id: string, repo: RepoFields) => {
      await repoClient.updateRepo({ repo: { id, ...repo } })
      await refresh()
    }
  }
}
