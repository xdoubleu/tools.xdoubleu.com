import { useEffect, useRef } from 'react'
import useSWR, { useSWRConfig } from 'swr'
import { swrKeys } from '@/lib/swrKeys'
import { createServiceClient } from '@/lib/client'
import { AgentAccountService } from '@/lib/gen/factory/v1/agent_accounts_pb'
import { RepoService, TaskService } from '@/lib/gen/factory/v1/factory_pb'
import { RunnerAdminService } from '@/lib/gen/factory/v1/runners_pb'
import type {
  GetTaskResponse,
  JobEvent,
  Harness,
  ListReposResponse,
  ListTasksResponse,
  TaskPriority,
  TaskStatus
} from '@/lib/gen/factory/v1/factory_pb'
import type { ListAgentAccountsResponse } from '@/lib/gen/factory/v1/agent_accounts_pb'
import type { ListRunnersResponse } from '@/lib/gen/factory/v1/runners_pb'
import { JOB_POLL_MS, jobActive } from '@/lib/factory/board'
import { LOG_POLL_MS } from '@/lib/factory/joblog'

export function useFactoryTasks() {
  const client = createServiceClient(TaskService)
  return useSWR<ListTasksResponse, Error>(swrKeys.factoryTasks, () => client.listTasks({}))
}

export function useFactoryRepos() {
  const client = createServiceClient(RepoService)
  return useSWR<ListReposResponse, Error>(swrKeys.factoryRepos, () => client.listRepos({}))
}

export function useFactoryAgents() {
  const client = createServiceClient(AgentAccountService)
  return useSWR<ListAgentAccountsResponse, Error>(swrKeys.factoryAgents, () =>
    client.listAgentAccounts({})
  )
}

/** How often the runner's state is refreshed: a little under its 45 s online window. */
export const RUNNER_POLL_MS = 15_000

/** The runners, refreshed on a timer so the board shows when one drops offline. */
export function useFactoryRunners() {
  const client = createServiceClient(RunnerAdminService)
  return useSWR<ListRunnersResponse, Error>(swrKeys.factoryRunners, () => client.listRunners({}), {
    refreshInterval: RUNNER_POLL_MS
  })
}

/** The release the served runner binary was built from; empty when none is published. */
export function useRunnerRelease() {
  return useSWR<string, Error>(swrKeys.factoryRunnerRelease, async () => {
    const res = await fetch(swrKeys.factoryRunnerRelease, { cache: 'no-store' })
    return res.ok ? (await res.text()).trim() : ''
  })
}

/** One task with its comments; no fetch without an id. */
export function useFactoryTask(id: string | null) {
  const client = createServiceClient(TaskService)
  return useSWR<GetTaskResponse, Error>(
    id ? swrKeys.factoryTask(id) : null,
    () => client.getTask({ id: id ?? '' }),
    // Refreshed while its newest job is on the way, so the sheet shows it finish.
    { refreshInterval: (data) => (jobActive(data?.jobs[0]) ? JOB_POLL_MS : 0) }
  )
}

/** The server returns at most this many events per call. */
const EVENTS_PAGE = 500

/**
 * Keeps the longest read of a job's log: overlapping fetches (a refresh while
 * one is in flight) finish in any order, and a slower one must not roll back.
 */
function remember(cache: Map<string, JobEvent[]>, jobId: string, events: JobEvent[]): JobEvent[] {
  const longest = cache.get(jobId)
  if (longest && longest.length > events.length) return longest
  cache.set(jobId, events)
  return events
}

/**
 * A job's log, fetched a page at a time after the last event seen, so a poll
 * costs what is new. Refreshed while the job is active, and once more when it
 * stops, to pick up what it said last.
 */
export function useJobLog(jobId: string | null, active: boolean) {
  const client = createServiceClient(TaskService)
  const kept = useRef(new Map<string, JobEvent[]>())

  const result = useSWR<JobEvent[], Error, ReturnType<typeof swrKeys.factoryJobEvents> | null>(
    jobId ? swrKeys.factoryJobEvents(jobId) : null,
    async ([, id]) => {
      let events = kept.current.get(id) ?? []
      for (;;) {
        const afterSeq = events.at(-1)?.seq ?? 0
        const page = (await client.listJobEvents({ jobId: id, afterSeq })).events
        events = remember(kept.current, id, [...events, ...page])
        if (page.length < EVENTS_PAGE) return events
      }
    },
    { refreshInterval: active ? LOG_POLL_MS : 0 }
  )

  const { mutate } = result
  const wasActive = useRef(active)
  useEffect(() => {
    if (wasActive.current && !active) void mutate()
    wasActive.current = active
  }, [active, mutate])

  return result
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
  const agentClient = createServiceClient(AgentAccountService)
  const runnerClient = createServiceClient(RunnerAdminService)
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
    /** Queues a job for the task; refused unless the body was last edited in a browser. */
    run: async (taskId: string, text: string) => {
      try {
        await client.runTask({ taskId, text })
      } finally {
        // A refusal still comments on the task and moves it to Needs you.
        await refresh()
      }
    },
    /** Queues a job that resumes the last job's session with the reply as its prompt. */
    reply: async (taskId: string, text: string) => {
      try {
        await client.replyTask({ taskId, text })
      } finally {
        await refresh()
      }
    },
    /** Cancels the task's queued or running job; the runner stops on its next check-in. */
    stop: async (taskId: string) => {
      await client.stopJob({ taskId })
      await refresh()
    },
    createRepo: async (repo: RepoFields) => {
      await repoClient.createRepo({ repo })
      await refresh()
    },
    updateRepo: async (id: string, repo: RepoFields) => {
      await repoClient.updateRepo({ repo: { id, ...repo } })
      await refresh()
    },
    /** Returns the new account and its client secret, which is shown once. */
    createAgent: async (name: string, totpCode: string) => {
      const created = await agentClient.createAgentAccount({ name, totpCode })
      await refresh()
      return created
    },
    revokeAgent: async (id: string) => {
      await agentClient.revokeAgentAccount({ id })
      await refresh()
    },
    /** Returns the new runner and its pairing code, which is shown once. */
    createRunner: async (name: string, totpCode: string) => {
      const created = await runnerClient.createRunner({ name, totpCode })
      await refresh()
      return created
    },
    revokeRunner: async (id: string) => {
      await runnerClient.revokeRunner({ id })
      await refresh()
    }
  }
}
