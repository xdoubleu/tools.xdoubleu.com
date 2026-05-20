import { renderHook } from '@testing-library/react'

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    createBugReport: jest.fn()
  }))
}))
jest.mock('@/lib/gen/bugreport/v1/bugreport_connect', () => ({
  BugReportService: {}
}))

import { createServiceClient } from '@/lib/client'
import { useCreateBugReport } from '@/hooks/useBugReport'

const mockCreateServiceClient = createServiceClient as jest.Mock

describe('useCreateBugReport', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('returns a function', () => {
    const { result } = renderHook(() => useCreateBugReport())
    expect(typeof result.current).toBe('function')
  })

  it('calls createServiceClient with BugReportService', () => {
    renderHook(() => useCreateBugReport())
    expect(mockCreateServiceClient).toHaveBeenCalledWith(expect.any(Object))
  })

  it('calls client.createBugReport with correct parameters', () => {
    const mockCreateBugReport = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      createBugReport: mockCreateBugReport
    })

    const { result } = renderHook(() => useCreateBugReport())
    result.current('Test Title', 'Test Description', '/test', 'console logs', 'ws logs')

    expect(mockCreateBugReport).toHaveBeenCalledWith({
      title: 'Test Title',
      description: 'Test Description',
      page: '/test',
      consoleLogs: 'console logs',
      wsLog: 'ws logs'
    })
  })

  it('returns promise from createBugReport', async () => {
    const mockResponse = { issueUrl: 'https://github.com/issue/123' }
    const mockCreateBugReport = jest.fn().mockResolvedValue(mockResponse)
    mockCreateServiceClient.mockReturnValue({
      createBugReport: mockCreateBugReport
    })

    const { result } = renderHook(() => useCreateBugReport())
    const response = await result.current('Title', 'Desc', '/page', '', '')

    expect(response).toEqual(mockResponse)
  })
})
