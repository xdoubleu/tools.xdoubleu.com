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
  sendWrite,
  subscribeOutbox,
  type WriteHandle
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
    revalidate: '/list',
    revalidateOnSuccess: true,
    // 'set' writes of `<key>:<n>` collapse per key.
    coalesceKey: (bytes: Uint8Array) => (name === 'set' ? decode(bytes).split(':')[0] : undefined)
  })
  return { shoppingListWrites: [make('add'), make('set')] }
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
  let handle: WriteHandle | undefined
  await act(async () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- the fake write encodes any JSON
    handle = await enqueueWrite(write, { value } as never)
  })
  return handle
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

    const handle = await enqueue('b')
    await act(() => flushOutbox())
    expect(handle?.status()).toBe('sent')

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(getOutboxSnapshot().pending).toBe(0)
    expect(screen.getByTestId('list')).toHaveTextContent('a,b')
  })

  it('keeps writes queued while offline and overlays them on fetched data', async () => {
    send.mockRejectedValue(networkError())

    const handle = await enqueue('b')
    await act(() => flushOutbox())

    expect(handle?.status()).toBe('queued')
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

    const handle = await enqueue('b')
    await act(() => flushOutbox())
    expect(handle?.status()).toBe('failed')

    expect(getOutboxSnapshot()).toMatchObject({
      pending: 0,
      failed: [{ description: 'Add b', reason: 'item not found' }]
    })

    await act(() => dismissFailed())
    expect(getOutboxSnapshot().failed).toEqual([])
  })

  it('sendWrite resolves to the send result and throws on rejection', async () => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- the fake write encodes any JSON
    const queue = (value: string) => enqueueWrite(write, { value } as never)

    await act(async () => expect(await sendWrite(queue('b'))).toBe('sent'))

    send.mockRejectedValueOnce(networkError())
    await act(async () => expect(await sendWrite(queue('c'))).toBe('queued'))

    send.mockRejectedValue(new ConnectError('item not found', Code.NotFound))
    await act(async () => {
      await expect(sendWrite(queue('d'))).rejects.toThrow('The server rejected the change')
    })
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

    const handle = await enqueue('b')
    await act(() => flushOutbox())
    expect(getOutboxSnapshot().pending).toBe(1)

    send.mockResolvedValue({})
    await act(() => flushOutbox())
    expect(handle?.status()).toBe('sent')
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

  it('loads stored writes and failures before the first overlay', async () => {
    await addQueued({
      writeId: write.id,
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- the fake write encodes any JSON
      request: write.encode({ value: 'b' } as never),
      createdAt: 1
    })
    await addQueued({ writeId: 'gone/Write', request: new Uint8Array(), createdAt: 1 })
    resetOutbox()

    expect(await applyPending('/list', ['a'])).toEqual(['a', 'b'])
  })

  it('shows failures stored by an earlier session once loaded', async () => {
    const { addFailed } = jest.requireMock<{ addFailed: (f: unknown) => Promise<void> }>(
      '@/lib/offline/store'
    )
    await addFailed({ description: 'Old', reason: 'gone' })
    resetOutbox()

    await act(() => flushOutbox())
    expect(getOutboxSnapshot().failed).toEqual([{ description: 'Old', reason: 'gone' }])
  })

  it('coalesces consecutive writes with the same key into the latest', async () => {
    const set: OfflineWrite = shoppingListWrites[1]
    const sendSet = jest.mocked(set.send)
    send.mockRejectedValue(networkError())
    sendSet.mockRejectedValue(networkError())
    const queue = (value: string) =>
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- the fake write encodes any JSON
      act(async () => void (await enqueueWrite(set, { value } as never)))

    await queue('k1:1')
    await queue('k1:2')
    await queue('k2:1')
    await queue('k2:2')
    await enqueue('b')
    await queue('k2:3')

    expect(getOutboxSnapshot().pending).toBe(4)
    expect(await applyPending('/list', [])).toEqual(['k1:2', 'k2:2', 'b', 'k2:3'])

    // The coalesced writes are gone from storage too.
    resetOutbox()
    expect(await applyPending('/list', [])).toEqual(['k1:2', 'k2:2', 'b', 'k2:3'])

    sendSet.mockReset().mockResolvedValue({})
    send.mockResolvedValue({})
    await act(() => flushOutbox())
    const sent = sendSet.mock.calls.map(([bytes]) => new TextDecoder().decode(bytes as Uint8Array))
    expect(sent).toEqual(['{"value":"k1:2"}', '{"value":"k2:2"}', '{"value":"k2:3"}'])
    expect(getOutboxSnapshot().pending).toBe(0)
  })

  it('never coalesces a write with its own copy reloaded by a concurrent drain', async () => {
    const set: OfflineWrite = shoppingListWrites[1]
    jest.mocked(set.send).mockRejectedValue(networkError())
    const queue = (value: string) =>
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- the fake write encodes any JSON
      act(async () => void (await enqueueWrite(set, { value } as never)))
    await queue('k1:1')
    const add = jest.mocked(addQueued)
    const store = add.getMockImplementation()!
    add.mockImplementationOnce(async (write) => {
      const seq = await store(write)
      await flushOutbox()
      return seq
    })

    await queue('k1:2')

    expect(getOutboxSnapshot().pending).toBe(1)
    resetOutbox()
    expect(await applyPending('/list', [])).toEqual(['k1:2'])
  })

  it('removes only the sent write from the queue', async () => {
    send.mockResolvedValueOnce({}).mockRejectedValue(networkError())

    await enqueue('b')
    await enqueue('c')
    await act(() => flushOutbox())

    expect(getOutboxSnapshot().pending).toBe(1)
    expect(await applyPending('/list', ['a'])).toEqual(['a', 'c'])
  })

  it('never gives up on network errors', async () => {
    send.mockRejectedValue(networkError())

    await enqueue('b')
    for (let i = 0; i < 6; i++) await act(() => flushOutbox())

    expect(getOutboxSnapshot()).toMatchObject({ pending: 1, failed: [] })
  })

  it('clears the expired-session state once a drain succeeds', async () => {
    send.mockRejectedValue(new ConnectError('expired', Code.Unauthenticated))

    await enqueue('b')
    await act(() => flushOutbox())
    expect(getOutboxSnapshot().authBlocked).toBe(true)

    send.mockResolvedValue({})
    await act(() => flushOutbox())
    expect(getOutboxSnapshot()).toMatchObject({ authBlocked: false, pending: 0 })
  })

  it('refetches only keys under the written app', async () => {
    const other = jest.fn(async () => ['x'])
    function OtherProbe() {
      const { data } = useSWR<string[]>('/other', other)
      return <span>{data?.join(',')}</span>
    }
    render(
      <SWRConfig value={{ dedupingInterval: 0 }}>
        <Probe />
        <OtherProbe />
      </SWRConfig>
    )
    await screen.findByText('x')
    await screen.findByText('a')

    await enqueue('b')
    await act(() => flushOutbox())

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(other).toHaveBeenCalledTimes(1)
  })

  it('refetches a write that already matches the server only when it is rejected', async () => {
    write.revalidateOnSuccess = false
    try {
      renderProbe()
      await screen.findByText('a')

      await enqueue('b')
      await act(async () => {
        await flushOutbox()
        await new Promise((resolve) => setTimeout(resolve, 50))
      })
      expect(fetcher).toHaveBeenCalledTimes(1)

      send.mockRejectedValue(new ConnectError('gone', Code.NotFound))
      await enqueue('c')
      await act(() => flushOutbox())
      await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    } finally {
      write.revalidateOnSuccess = true
    }
  })

  it('reset forgets queued writes, failures and the expired session', async () => {
    send.mockRejectedValue(new ConnectError('expired', Code.Unauthenticated))
    await enqueue('b')
    await act(() => flushOutbox())

    resetOutbox()

    expect(getOutboxSnapshot()).toEqual({ pending: 0, failed: [], authBlocked: false })
  })
})
