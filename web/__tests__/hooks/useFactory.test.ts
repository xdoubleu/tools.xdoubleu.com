import { renderHook } from '@testing-library/react'

const sampleKeys = [
  '/factory/tasks',
  '/factory/repos',
  ['/factory/task', 't-1'],
  '/podcasts/favourites',
  undefined
]
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
  addComment: vi.fn().mockResolvedValue({}),
  listRepos: vi.fn().mockResolvedValue({ repos: [] }),
  createRepo: vi.fn().mockResolvedValue({}),
  updateRepo: vi.fn().mockResolvedValue({})
}
vi.mock('@/lib/client', () => ({
  createServiceClient: vi.fn(() => mockClient)
}))
vi.mock('@/lib/gen/factory/v1/factory_pb', () => ({ TaskService: {}, RepoService: {} }))

import useSWR from 'swr'
import {
  useFactoryActions,
  useFactoryRepos,
  useFactoryTask,
  useFactoryTasks
} from '@/hooks/useFactory'

const mockUseSWR = vi.mocked(useSWR)
const fields = { title: 't', body: 'b', priority: 3, labels: ['x'] }
const repoFields = {
  owner: 'o',
  name: 'n',
  defaultBranch: 'main',
  defaultHarness: 1,
  setupCommands: 'make',
  agentInstructions: 'be nice',
  timeLimitMinutes: 30
}

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

describe('useFactoryRepos', () => {
  it('fetches the repos', () => {
    renderHook(() => useFactoryRepos())
    expect(mockUseSWR).toHaveBeenCalledWith('/factory/repos', expect.any(Function))
    expect(mockClient.listRepos).toHaveBeenCalledWith({})
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
    const task = await result.current.create({ ...fields, repoId: 'r-1' })
    expect(mockClient.createTask).toHaveBeenCalledWith({ ...fields, repoId: 'r-1' })
    expect(task).toEqual({ id: 'new' })
    expect(revalidated).toEqual(['/factory/tasks', '/factory/repos', ['/factory/task', 't-1']])
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

  it('adds and edits repos, each revalidating', async () => {
    const { result } = renderHook(() => useFactoryActions())

    await result.current.createRepo(repoFields)
    expect(mockClient.createRepo).toHaveBeenCalledWith({ repo: repoFields })

    await result.current.updateRepo('r-1', repoFields)
    expect(mockClient.updateRepo).toHaveBeenCalledWith({ repo: { id: 'r-1', ...repoFields } })

    expect(mockMutate).toHaveBeenCalledTimes(2)
  })
})
