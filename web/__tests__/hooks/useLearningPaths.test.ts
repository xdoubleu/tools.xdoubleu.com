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

import useSWR from 'swr'
import {
  useLearningPaths,
  useLearningPath,
  useCreateLearningPath,
  useUpdateLearningPath,
  useDeleteLearningPath,
  useRecordItemProgress,
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

  it('useRecordItemProgress returns a function', () => {
    const { result } = renderHook(() => useRecordItemProgress())
    expect(typeof result.current).toBe('function')
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
