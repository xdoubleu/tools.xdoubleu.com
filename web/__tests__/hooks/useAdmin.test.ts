import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    listUsers: jest.fn(),
    setRole: jest.fn(),
    setAppAccess: jest.fn()
  }))
}))
jest.mock('@/lib/gen/admin/v1/admin_pb', () => ({
  AdminService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { useUsers, useSetRole, useSetAppAccess } from '@/hooks/useAdmin'

const mockUseSWR = jest.mocked(useSWR)
const mockCreateServiceClient = jest.mocked(createServiceClient)

beforeEach(() => {
  mockUseSWR.mockReturnValue(
    // @ts-expect-error -- mock returns a partial SWRResponse for test purposes
    { data: undefined, isLoading: false, error: undefined }
  )
  mockUseSWR.mockClear()
})

describe('useUsers', () => {
  it('uses /admin/users as key', () => {
    renderHook(() => useUsers())
    expect(mockUseSWR).toHaveBeenCalledWith('/admin/users', expect.any(Function))
  })

  it('returns SWR result', () => {
    const mockData = { users: [{ id: '1', email: 'a@b.com', role: 'user' }] }
    mockUseSWR.mockReturnValueOnce(
      // @ts-expect-error -- mock returns a partial SWRResponse for test purposes
      { data: mockData, isLoading: false, error: undefined }
    )
    const { result } = renderHook(() => useUsers())
    expect(result.current.data).toEqual(mockData)
  })
})

describe('useSetRole', () => {
  it('returns a function that calls client.setRole', () => {
    const mockSetRole = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ setRole: mockSetRole })

    const { result } = renderHook(() => useSetRole())
    result.current('user-1', 'admin')
    expect(mockSetRole).toHaveBeenCalledWith({ userId: 'user-1', role: 'admin' })
  })
})

describe('useSetAppAccess', () => {
  it('returns a function that calls client.setAppAccess', () => {
    const mockSetAppAccess = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ setAppAccess: mockSetAppAccess })

    const { result } = renderHook(() => useSetAppAccess())
    result.current('user-1', 'backlog', true)
    expect(mockSetAppAccess).toHaveBeenCalledWith({
      userId: 'user-1',
      appName: 'backlog',
      grant: true
    })
  })
})
