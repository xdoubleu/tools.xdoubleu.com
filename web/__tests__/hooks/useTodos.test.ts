import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({}))
}))
jest.mock('@/lib/gen/todos/v1/tasks_pb', () => ({
  TaskService: {}
}))
jest.mock('@/lib/gen/todos/v1/subtasks_pb', () => ({
  SubtaskService: {}
}))
jest.mock('@/lib/gen/todos/v1/settings_pb', () => ({
  SettingsService: {}
}))

import useSWR from 'swr'
import { useTodos } from '@/hooks/useTodos'
import { useTodoSettings } from '@/hooks/useTodoSettings'

const mockUseSWR = jest.mocked(useSWR)

beforeEach(() => {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  mockUseSWR.mockClear()
})

describe('useTodos', () => {
  it('passes "/todos" as key when no params given', () => {
    renderHook(() => useTodos())
    expect(mockUseSWR).toHaveBeenCalledWith('/todos', expect.any(Function))
  })

  it('passes array key when query params given', () => {
    renderHook(() => useTodos({ workspaceId: 'ws-1' }))
    const args = mockUseSWR.mock.calls[0]!
    const key = args[0]
    expect(Array.isArray(key)).toBe(true)
    if (Array.isArray(key)) {
      expect(key[0]).toBe('/todos')
    }
  })

  it('returns the SWR result', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValueOnce({
      data: { tasks: [{ id: '1' }] },
      isLoading: false,
      error: undefined
    })
    const { result } = renderHook(() => useTodos())
    expect(result.current.data).toEqual({ tasks: [{ id: '1' }] })
  })
})

describe('useTodoSettings', () => {
  it('passes "/todos/settings" as key', () => {
    renderHook(() => useTodoSettings())
    expect(mockUseSWR).toHaveBeenCalledWith('/todos/settings', expect.any(Function))
  })

  it('returns the SWR result', () => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValueOnce({ data: { settings: {} }, isLoading: false, error: undefined })
    const { result } = renderHook(() => useTodoSettings())
    expect(result.current.data).toEqual({ settings: {} })
  })
})
