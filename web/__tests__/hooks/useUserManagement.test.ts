import { renderHook } from '@testing-library/react'

const mockMutate = vi.fn()
vi.mock('swr', () => ({
  __esModule: true,
  default: vi.fn(),
  mutate: (...args: unknown[]) => mockMutate(...args)
}))
vi.mock('@/lib/client', () => ({
  createServiceClient: vi.fn(() => ({
    listUsers: vi.fn(),
    setRole: vi.fn(),
    setAppAccess: vi.fn()
  }))
}))
vi.mock('@/lib/gen/access/v1/access_pb', () => ({
  AccessService: {}
}))
vi.mock('@/lib/gen/factory/v1/agent_accounts_pb', () => ({ AgentAccountService: {} }))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useAgentActions,
  useAgents,
  useUsers,
  useSetRole,
  useSetAppAccess,
  useSetDisplayName
} from '@/hooks/useUserManagement'

const mockUseSWR = vi.mocked(useSWR)
const mockCreateServiceClient = vi.mocked(createServiceClient)

beforeEach(() => {
  mockUseSWR.mockReturnValue(
    // @ts-expect-error -- mock returns a partial SWRResponse for test purposes
    { data: undefined, isLoading: false, error: undefined }
  )
  mockUseSWR.mockClear()
})

describe('useUsers', () => {
  it('uses /user-management/users as key', () => {
    renderHook(() => useUsers())
    expect(mockUseSWR).toHaveBeenCalledWith('/user-management/users', expect.any(Function))
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
    const mockSetRole = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ setRole: mockSetRole })

    const { result } = renderHook(() => useSetRole())
    result.current('user-1', 'admin')
    expect(mockSetRole).toHaveBeenCalledWith({ userId: 'user-1', role: 'admin' })
  })
})

describe('useSetAppAccess', () => {
  it('returns a function that calls client.setAppAccess', () => {
    const mockSetAppAccess = vi.fn().mockResolvedValue({})
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

describe('useSetDisplayName', () => {
  it('returns a function that calls client.setDisplayName', () => {
    const setDisplayName = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ setDisplayName })

    const { result } = renderHook(() => useSetDisplayName())
    result.current('user-1', 'Ada Lovelace')
    expect(setDisplayName).toHaveBeenCalledWith({ userId: 'user-1', displayName: 'Ada Lovelace' })
  })
})

describe('useAgents', () => {
  it('lists the agent accounts under its own key', () => {
    const listAgentAccounts = vi.fn().mockResolvedValue({ accounts: [] })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ listAgentAccounts })

    // @ts-expect-error -- mock returns a partial SWRResponse for test purposes
    mockUseSWR.mockImplementationOnce((_key: unknown, fetcher: unknown) => {
      if (typeof fetcher === 'function') void fetcher()
      return { data: undefined, isLoading: false, error: undefined }
    })

    renderHook(() => useAgents())
    expect(mockUseSWR).toHaveBeenCalledWith('/user-management/agents', expect.any(Function))
    expect(listAgentAccounts).toHaveBeenCalledWith({})
  })
})

describe('useAgentActions', () => {
  const client = {
    createAgentAccount: vi.fn().mockResolvedValue({ clientSecret: 's3cret' }),
    revokeAgentAccount: vi.fn().mockResolvedValue({}),
    rotateAgentAccountSecret: vi.fn().mockResolvedValue({ clientSecret: 'n3w' }),
    addAgentAccountToFamily: vi.fn().mockResolvedValue({}),
    removeAgentAccountFromFamily: vi.fn().mockResolvedValue({}),
    setAgentAppAccess: vi.fn().mockResolvedValue({})
  }

  beforeEach(() => {
    mockMutate.mockClear()
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue(client)
  })

  it('creates and rotates, returning the one-time secret', async () => {
    const { result } = renderHook(() => useAgentActions())

    expect(await result.current.create('Reviewer')).toEqual({ clientSecret: 's3cret' })
    expect(client.createAgentAccount).toHaveBeenCalledWith({ name: 'Reviewer' })

    expect(await result.current.rotate('a-1', '654321')).toEqual({ clientSecret: 'n3w' })
    expect(client.rotateAgentAccountSecret).toHaveBeenCalledWith({ id: 'a-1', totpCode: '654321' })
    expect(mockMutate).toHaveBeenCalledTimes(2)
    expect(mockMutate).toHaveBeenCalledWith('/user-management/agents')
  })

  it('revokes and moves an agent in and out of the family', async () => {
    const { result } = renderHook(() => useAgentActions())

    await result.current.revoke('a-1')
    expect(client.revokeAgentAccount).toHaveBeenCalledWith({ id: 'a-1' })
    await result.current.joinFamily('a-1', '123456')
    expect(client.addAgentAccountToFamily).toHaveBeenCalledWith({ id: 'a-1', totpCode: '123456' })
    await result.current.leaveFamily('a-1')
    expect(client.removeAgentAccountFromFamily).toHaveBeenCalledWith({ id: 'a-1' })
    expect(mockMutate).toHaveBeenCalledTimes(3)
  })

  it('sets an app grant for the agent user', async () => {
    const { result } = renderHook(() => useAgentActions())

    await result.current.setAccess('u-1', 'books', 'write')
    expect(client.setAgentAppAccess).toHaveBeenCalledWith({
      userId: 'u-1',
      appName: 'books',
      level: 'write'
    })
    expect(mockMutate).toHaveBeenCalledWith('/user-management/agents')
  })
})
