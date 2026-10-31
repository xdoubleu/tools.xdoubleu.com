import { renderHook } from '@testing-library/react'

const sampleKeys = ['/factory/tasks', ['/factory/task', 't-1'], '/podcasts/favourites', undefined]
let revalidated: unknown[] = []
const mockMutate = vi.fn(async (match: (key: unknown) => boolean) => {
  revalidated = sampleKeys.filter(match)
})
vi.mock('swr', () => ({
  __esModule: true,
  default: vi.fn(),
  useSWRConfig: () => ({ mutate: mockMutate })
}))
const mockClient = {
  listTasks: vi.fn().mockResolvedValue({ tasks: [] }),
  getTask: vi.fn().mockResolvedValue({ comments: [] }),
  createTask: vi.fn().mockResolvedValue({ task: { id: 'new' } }),
  updateTask: vi.fn().mockResolvedValue({}),
  moveTask: vi.fn().mockResolvedValue({}),
  addComment: vi.fn().mockResolvedValue({})
}
vi.mock('@/lib/client', () => ({
  createServiceClient: vi.fn(() => mockClient)
}))
vi.mock('@/lib/gen/factory/v1/factory_pb', () => ({ TaskService: {} }))

import useSWR from 'swr'
import { useFactoryActions, useFactoryTask, useFactoryTasks } from '@/hooks/useFactory'

const mockUseSWR = vi.mocked(useSWR)
const fields = { title: 't', body: 'b', priority: 3, labels: ['x'] }

beforeEach(() => {
  vi.clearAllMocks()
  revalidated = []
  // Runs the fetcher for a non-null key, like SWR would.
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockImplementation((key: unknown, fetcher: unknown) => {
    if (key !== null && typeof fetcher === 'function') void fetcher()
    return { data: undefined, isLoading: false, error: undefined }
  })
})

describe('useFactoryTasks', () => {
  it('fetches the board', () => {
    renderHook(() => useFactoryTasks())
    expect(mockUseSWR).toHaveBeenCalledWith('/factory/tasks', expect.any(Function))
    expect(mockClient.listTasks).toHaveBeenCalledWith({})
  })
})

describe('useFactoryTask', () => {
  it('fetches one task', () => {
    renderHook(() => useFactoryTask('t-1'))
    expect(mockUseSWR).toHaveBeenCalledWith(['/factory/task', 't-1'], expect.any(Function))
    expect(mockClient.getTask).toHaveBeenCalledWith({ id: 't-1' })
  })

  it('does not fetch without an id', () => {
    renderHook(() => useFactoryTask(null))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
    expect(mockClient.getTask).not.toHaveBeenCalled()
  })
})

describe('useFactoryActions', () => {
  it('creates, returns the task and revalidates only factory keys', async () => {
    const { result } = renderHook(() => useFactoryActions())
    const task = await result.current.create(fields)
    expect(mockClient.createTask).toHaveBeenCalledWith(fields)
    expect(task).toEqual({ id: 'new' })
    expect(revalidated).toEqual(['/factory/tasks', ['/factory/task', 't-1']])
  })

  it('updates, moves and comments, each revalidating', async () => {
    const { result } = renderHook(() => useFactoryActions())

    await result.current.update('t-1', fields)
    expect(mockClient.updateTask).toHaveBeenCalledWith({ id: 't-1', ...fields })

    await result.current.move('t-1', 2)
    expect(mockClient.moveTask).toHaveBeenCalledWith({ id: 't-1', status: 2 })

    await result.current.comment('t-1', 'hi')
    expect(mockClient.addComment).toHaveBeenCalledWith({ taskId: 't-1', body: 'hi' })

    expect(mockMutate).toHaveBeenCalledTimes(3)
  })
})
