import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
const mockClient = {
  listLearningPaths: jest.fn().mockResolvedValue({ learningPaths: [{ id: 'lp-1' }], hasMore: true })
}
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => mockClient)
}))
jest.mock('@/lib/gen/learningpaths/v1/learningpaths_pb', () => ({
  LearningPathsService: {}
}))
jest.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: jest.fn().mockResolvedValue({ status: () => 'sent' }),
  flushOutbox: jest.fn().mockResolvedValue(undefined)
}))
jest.mock('@/lib/learningpaths/offlineWrites', () => ({
  recordItemProgressWrite: { id: 'progress' },
  setPausedWrite: { id: 'paused' }
}))

import useSWR from 'swr'
import { enqueueWrite, flushOutbox } from '@/lib/offline/outbox'
import { recordItemProgressWrite, setPausedWrite } from '@/lib/learningpaths/offlineWrites'
import {
  useLearningPaths,
  useLearningPath,
  useCreateLearningPath,
  useUpdateLearningPath,
  useDeleteLearningPath,
  useRecordItemProgress,
  useSetLearningPathPaused,
  useFetchLearningPathsPage
} from '@/hooks/useLearningPaths'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useLearningPaths', () => {
  it('uses /learningpaths as key', () => {
    renderHook(() => useLearningPaths())
    expect(mockUseSWR).toHaveBeenCalledWith('/learningpaths', expect.any(Function))
  })
})

describe('useLearningPath', () => {
  it('uses /learningpaths/:id as key when id is given', () => {
    renderHook(() => useLearningPath('lp-1'))
    expect(mockUseSWR).toHaveBeenCalledWith('/learningpaths/lp-1', expect.any(Function))
  })

  it('passes null as key when id is empty', () => {
    renderHook(() => useLearningPath(''))
    expect(mockUseSWR).toHaveBeenCalledWith(null, expect.any(Function))
  })
})

describe('mutation hooks return functions', () => {
  it('useCreateLearningPath returns a function', () => {
    const { result } = renderHook(() => useCreateLearningPath())
    expect(typeof result.current).toBe('function')
  })

  it('useUpdateLearningPath returns a function', () => {
    const { result } = renderHook(() => useUpdateLearningPath())
    expect(typeof result.current).toBe('function')
  })

  it('useDeleteLearningPath returns a function', () => {
    const { result } = renderHook(() => useDeleteLearningPath())
    expect(typeof result.current).toBe('function')
  })
})

describe('progress and pause go through the outbox', () => {
  it('useRecordItemProgress queues the progress and sends it', async () => {
    const { result } = renderHook(() => useRecordItemProgress())
    await result.current({ itemId: 'i-1', completed: true })
    expect(enqueueWrite).toHaveBeenCalledWith(recordItemProgressWrite, {
      itemId: 'i-1',
      completed: true
    })
    expect(flushOutbox).toHaveBeenCalled()
  })

  it('useSetLearningPathPaused queues the paused flag for the path', async () => {
    const { result } = renderHook(() => useSetLearningPathPaused())
    await result.current({ id: 'lp-1', paused: true })
    expect(enqueueWrite).toHaveBeenCalledWith(setPausedWrite, { id: 'lp-1', paused: true })
  })
})

describe('useFetchLearningPathsPage', () => {
  it('fetches a page at the given offset and maps the response', async () => {
    const { result } = renderHook(() => useFetchLearningPathsPage())
    const page = await result.current(50)
    expect(mockClient.listLearningPaths).toHaveBeenCalledWith({ limit: 50, offset: 50 })
    expect(page).toEqual({ items: [{ id: 'lp-1' }], hasMore: true })
  })
})
