import type { DescMessage, MessageInitShape } from '@bufbuild/protobuf'
import { Code, ConnectError } from '@connectrpc/connect'
import { mutate, type Arguments } from 'swr'
import { bookWrites } from '@/lib/books/offlineWrites'
import { feedWrites } from '@/lib/feeds/offlineWrites'
import { learningPathWrites } from '@/lib/learningpaths/offlineWrites'
import { mealPlanWrites } from '@/lib/mealplans/offlineWrites'
import { recipeWrites } from '@/lib/recipes/offlineWrites'
import { shoppingListWrites } from '@/lib/shoppinglist/offlineWrites'
import { isNetworkError } from './network'
import type { OfflineWrite } from './registry'
import {
  addFailed,
  addQueued,
  clearFailed,
  deleteQueued,
  listFailed,
  listQueued,
  type FailedWrite,
  type QueuedWrite
} from './store'

const writes: OfflineWrite[] = [
  ...shoppingListWrites,
  ...recipeWrites,
  ...mealPlanWrites,
  ...feedWrites,
  ...bookWrites,
  ...learningPathWrites
]
const registry = new Map(writes.map((w) => [w.id, w]))

// A write failing with a server error this many times is given up on.
const MAX_ATTEMPTS = 5
const RETRYABLE = new Set([
  Code.Unknown,
  Code.Internal,
  Code.DeadlineExceeded,
  Code.ResourceExhausted,
  Code.Aborted
])

export interface OutboxState {
  pending: number
  failed: FailedWrite[]
  /** The session expired; queued writes wait for the user to sign in. */
  authBlocked: boolean
}

const EMPTY: OutboxState = { pending: 0, failed: [], authBlocked: false }

let state: OutboxState = EMPTY
const listeners = new Set<() => void>()
// Mirrors the IndexedDB queue, oldest first; entries without `seq` exist only
// here (IndexedDB unavailable).
let queue: QueuedWrite[] = []
// Keyed by seq, which survives the reload at the start of every drain.
const attempts = new Map<number | QueuedWrite, number>()
const settled = new Map<number | QueuedWrite, 'sent' | 'failed'>()

export type WriteStatus = 'queued' | 'sent' | 'failed'

/** Reports a queued write's progress, as of the last drain. */
export interface WriteHandle {
  status(): WriteStatus
}
let loading: Promise<void> | null = null
let flushing: Promise<void> | null = null

function publish(next: Partial<OutboxState>) {
  state = { ...state, pending: queue.length, ...next }
  listeners.forEach((l) => l())
}

export function subscribeOutbox(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function getOutboxSnapshot(): OutboxState {
  return state
}

export function getOutboxServerSnapshot(): OutboxState {
  return EMPTY
}

async function reload() {
  const [stored, failed] = await Promise.all([listQueued(), listFailed()])
  queue = [...stored, ...queue.filter((w) => w.seq === undefined)]
  publish({ failed })
}

function outboxReady(): Promise<void> {
  loading ??= reload()
  return loading
}

/** Re-applies queued writes to data just fetched or loaded for `key`. */
export async function applyPending<T>(key: unknown, data: T): Promise<T> {
  await outboxReady()
  let result: unknown = data
  for (const write of queue) {
    const def = registry.get(write.writeId)
    if (def) result = def.apply(key, result, write.request, write.hint)
  }
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- reducers return data of the shape they were given
  return result as T
}

function keyPath(key: unknown): unknown {
  return Array.isArray(key) ? key[0] : key
}

async function applyOptimistic(def: OfflineWrite, write: QueuedWrite) {
  const keys: Arguments[] = []
  await mutate((key) => {
    keys.push(key)
    return false
  })
  await Promise.all(
    keys.map((key) =>
      mutate(
        key,
        (data: unknown) =>
          data === undefined ? data : def.apply(key, data, write.request, write.hint),
        { revalidate: false }
      )
    )
  )
}

/**
 * Queues a write, applies it to the SWR cache at once, and sends it when the
 * API is reachable. Never throws: rejected writes land in `failed`.
 */
export async function enqueueWrite<I extends DescMessage>(
  def: OfflineWrite<I>,
  init: MessageInitShape<I>,
  hint?: unknown
): Promise<WriteHandle> {
  await outboxReady()
  const write: QueuedWrite = {
    writeId: def.id,
    request: def.encode(init),
    hint,
    createdAt: Date.now()
  }
  write.seq = await addQueued(write)
  queue.push(write)
  publish({})
  await applyOptimistic(def, write)
  void flushOutbox()
  return { status: () => settled.get(write.seq ?? write) ?? 'queued' }
}

async function remove(write: QueuedWrite) {
  queue = queue.filter((w) => w !== write)
  if (write.seq !== undefined) await deleteQueued(write.seq)
}

async function reject(write: QueuedWrite, failure: FailedWrite) {
  settled.set(write.seq ?? write, 'failed')
  await remove(write)
  await addFailed(failure)
  publish({ failed: [...state.failed, failure] })
}

async function drain() {
  await outboxReady()
  await reload()
  publish({ authBlocked: false })
  const refetch = new Set<string>()
  while (queue.length > 0) {
    const write = queue[0]
    const def = registry.get(write.writeId)
    if (!def) {
      await reject(write, { description: 'An unknown change', reason: 'unsupported' })
      continue
    }
    try {
      await def.send(write.request)
      settled.set(write.seq ?? write, 'sent')
      await remove(write)
      if (def.revalidateOnSuccess) refetch.add(def.revalidate)
    } catch (err) {
      if (isNetworkError(err)) break
      const code = err instanceof ConnectError ? err.code : Code.Unknown
      if (code === Code.Unauthenticated) {
        publish({ authBlocked: true })
        break
      }
      const tries = (attempts.get(write.seq ?? write) ?? 0) + 1
      attempts.set(write.seq ?? write, tries)
      if (RETRYABLE.has(code) && tries < MAX_ATTEMPTS) break
      await reject(write, {
        description: def.describe(write.request),
        reason: err instanceof ConnectError ? err.rawMessage : String(err)
      })
      refetch.add(def.revalidate)
    }
    publish({})
  }
  for (const prefix of refetch) {
    void mutate((key) => {
      const path = keyPath(key)
      return typeof path === 'string' && path.startsWith(prefix)
    })
  }
}

/** Sends queued writes in order; one drain at a time, across tabs too. */
export function flushOutbox(): Promise<void> {
  flushing ??= (
    typeof navigator !== 'undefined' && 'locks' in navigator
      ? navigator.locks.request('tools-outbox', drain)
      : drain()
  ).finally(() => {
    flushing = null
  })
  return flushing
}

/** Sends a just-queued write; throws if the server rejected it. */
export async function sendWrite(pending: Promise<WriteHandle>): Promise<WriteStatus> {
  const handle = await pending
  await flushOutbox()
  const status = handle.status()
  if (status === 'failed') throw new Error('The server rejected the change')
  return status
}

export async function dismissFailed(): Promise<void> {
  await clearFailed()
  publish({ failed: [] })
}

/** Forgets the in-memory queue after the stored one was wiped. */
export function resetOutbox() {
  queue = []
  loading = null
  publish({ failed: [], authBlocked: false })
}
