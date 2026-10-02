import { act, render, screen, waitFor } from '@testing-library/react'
import useSWR, { mutate, SWRConfig } from 'swr'
import { Code, ConnectError } from '@connectrpc/connect'
import {
  applyPending,
  dismissFailed,
  enqueueWrite,
  flushOutbox,
  getOutboxServerSnapshot,
  getOutboxSnapshot,
  resetOutbox,
  subscribeOutbox
} from '@/lib/offline/outbox'
import { shoppingListWrites } from '@/lib/shoppinglist/offlineWrites'
import { addQueued, type QueuedWrite } from '@/lib/offline/store'
import type { OfflineWrite } from '@/lib/offline/registry'

// Writes that append their JSON `value` to cached '/list…' arrays.
jest.mock('@/lib/shoppinglist/offlineWrites', () => {
  const decode = (bytes: Uint8Array): string => {
    const parsed: unknown = JSON.parse(new TextDecoder().decode(bytes))
    return typeof parsed === 'object' && parsed !== null && 'value' in parsed
      ? String(parsed.value)
      : ''
  }
  const make = (name: string) => ({
    id: `test/${name}`,
    encode: (init: unknown) => new TextEncoder().encode(JSON.stringify(init)),
    send: jest.fn(async () => ({})),
    apply: (key: unknown, data: unknown, bytes: Uint8Array) => {
      const value = decode(bytes)
      return typeof key === 'string' && key.startsWith('/list') && Array.isArray(data)
        ? [...data.filter((v) => v !== value), value]
        : data
    },
    describe: (bytes: Uint8Array) => `Add ${decode(bytes)}`,
    revalidate: '/list'
  })
  return { shoppingListWrites: [make('add')] }
})

jest.mock('@/lib/offline/store', () => {
  let queued: Record<string, unknown>[] = []
  let failed: Record<string, unknown>[] = []
  let seq = 0
  return {
    mockReset: () => {
      queued = []
      failed = []
    },
    mockMemoryOnly: { on: false },
    addQueued: jest.fn(async function (this: unknown, write: Record<string, unknown>) {
      const self = jest.requireMock<{ mockMemoryOnly: { on: boolean } }>('@/lib/offline/store')
      if (self.mockMemoryOnly.on) return undefined
      seq++
      queued.push({ ...write, seq })
      return seq
    }),
    listQueued: jest.fn(async () => queued.map((w) => ({ ...w }))),
    deleteQueued: jest.fn(async (s: number) => {
      queued = queued.filter((w) => w.seq !== s)
    }),
    addFailed: jest.fn(async (f: Record<string, unknown>) => {
      failed.push(f)
    }),
    listFailed: jest.fn(async () => [...failed]),
    clearFailed: jest.fn(async () => {
      failed = []
    })
  }
})

const storeMock = jest.requireMock<{
  mockReset: () => void
  mockMemoryOnly: { on: boolean }
}>('@/lib/offline/store')

const write: OfflineWrite = shoppingListWrites[0]
const send = jest.mocked(write.send)
const fetcher = jest.fn<Promise<string[]>, []>()

// A fresh key per test, so no test joins another's in-flight refetch.
let listKey = '/list'

function Probe() {
  const { data } = useSWR<string[]>(listKey, fetcher)
  return <span data-testid="list">{data?.join(',') ?? 'loading'}</span>
}

function renderProbe() {
  return render(
    <SWRConfig value={{ dedupingInterval: 0 }}>
      <Probe />
    </SWRConfig>
  )
}

const networkError = () => new TypeError('Failed to fetch')

async function enqueue(value: string) {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- the fake write encodes any JSON
  await act(() => enqueueWrite(write, { value } as never))
}

describe('outbox', () => {
  let testIndex = 0

  beforeEach(async () => {
    listKey = `/list/${++testIndex}`
    // The outbox mutates SWR's global cache, as the app does.
    await mutate(() => true, undefined, { revalidate: false })
    storeMock.mockReset()
    storeMock.mockMemoryOnly.on = false
    resetOutbox()
    jest.clearAllMocks()
    send.mockResolvedValue({})
    fetcher.mockResolvedValue(['a'])
  })

  it('applies a write to the cache before it is sent', async () => {
    let finishSend = () => {}
    send.mockImplementation(
      () =>
        new Promise((resolve) => {
          finishSend = () => resolve({})
        })
    )
    renderProbe()
    expect(await screen.findByText('a')).toBeInTheDocument()

    await enqueue('b')

    expect(screen.getByTestId('list')).toHaveTextContent('a,b')
    expect(getOutboxSnapshot().pending).toBe(1)
    expect(send).toHaveBeenCalledTimes(1)
    await act(async () => {
      finishSend()
      await flushOutbox()
    })
  })

  it('removes a sent write and refetches its keys', async () => {
    renderProbe()
    await screen.findByText('a')
    fetcher.mockResolvedValue(['a', 'b'])

    await enqueue('b')
    await act(() => flushOutbox())

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(getOutboxSnapshot().pending).toBe(0)
    expect(screen.getByTestId('list')).toHaveTextContent('a,b')
  })

  it('keeps writes queued while offline and overlays them on fetched data', async () => {
    send.mockRejectedValue(networkError())

    await enqueue('b')
    await act(() => flushOutbox())

    expect(getOutboxSnapshot().pending).toBe(1)
    expect(await applyPending('/list', ['a'])).toEqual(['a', 'b'])
    expect(await applyPending('/other', ['a'])).toEqual(['a'])

    send.mockResolvedValue({})
    await act(() => flushOutbox())
    expect(getOutboxSnapshot().pending).toBe(0)
  })

  it('pauses on an expired session', async () => {
    send.mockRejectedValue(new ConnectError('expired', Code.Unauthenticated))

    await enqueue('b')
    await act(() => flushOutbox())

    expect(getOutboxSnapshot()).toMatchObject({ pending: 1, authBlocked: true })
  })

  it('moves a rejected write to the failed list until dismissed', async () => {
    send.mockRejectedValue(new ConnectError('item not found', Code.NotFound))

    await enqueue('b')
    await act(() => flushOutbox())

    expect(getOutboxSnapshot()).toMatchObject({
      pending: 0,
      failed: [{ description: 'Add b', reason: 'item not found' }]
    })

    await act(() => dismissFailed())
    expect(getOutboxSnapshot().failed).toEqual([])
  })

  it('retries server errors, then gives up after five attempts', async () => {
    send.mockRejectedValue(new ConnectError('boom', Code.Internal))

    await enqueue('b')
    for (let i = 0; i < 3; i++) await act(() => flushOutbox())
    expect(getOutboxSnapshot().pending).toBe(1)

    await act(() => flushOutbox())
    expect(getOutboxSnapshot()).toMatchObject({
      pending: 0,
      failed: [{ description: 'Add b', reason: 'boom' }]
    })
  })

  it('reports non-Connect errors by message', async () => {
    send.mockRejectedValue('weird')

    await enqueue('b')
    for (let i = 0; i < 4; i++) await act(() => flushOutbox())

    expect(getOutboxSnapshot().failed).toEqual([{ description: 'Add b', reason: 'weird' }])
  })

  it('fails a stored write no registered write can send', async () => {
    const unknown: QueuedWrite = { writeId: 'gone/Write', request: new Uint8Array(), createdAt: 1 }
    await addQueued(unknown)

    await act(() => flushOutbox())

    expect(getOutboxSnapshot().failed).toEqual([
      { description: 'An unknown change', reason: 'unsupported' }
    ])
  })

  it('keeps writes in memory when IndexedDB is unavailable', async () => {
    storeMock.mockMemoryOnly.on = true
    send.mockRejectedValue(networkError())

    await enqueue('b')
    await act(() => flushOutbox())

    expect(getOutboxSnapshot().pending).toBe(1)
  })

  it('runs one drain at a time, under a cross-tab lock when available', async () => {
    const request = jest.fn((_name: string, fn: () => Promise<void>) => fn())
    Object.defineProperty(navigator, 'locks', { value: { request }, configurable: true })
    try {
      const first = flushOutbox()
      expect(flushOutbox()).toBe(first)
      await act(() => first)
      expect(request).toHaveBeenCalledWith('tools-outbox', expect.any(Function))
    } finally {
      // @ts-expect-error -- restore jsdom's navigator without locks
      delete navigator.locks
    }
  })

  it('notifies subscribers and reports nothing pending on the server', async () => {
    const listener = jest.fn()
    const unsubscribe = subscribeOutbox(listener)
    send.mockRejectedValue(networkError())

    await enqueue('b')
    expect(listener).toHaveBeenCalled()

    unsubscribe()
    listener.mockClear()
    resetOutbox()
    expect(listener).not.toHaveBeenCalled()
    expect(getOutboxServerSnapshot()).toEqual({ pending: 0, failed: [], authBlocked: false })
  })
})
