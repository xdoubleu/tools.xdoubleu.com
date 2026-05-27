import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    signIn: jest.fn(),
    signOut: jest.fn(),
    forgotPassword: jest.fn(),
    mFAChallenge: jest.fn(),
    getCurrentUser: jest.fn()
  }))
}))
jest.mock('@/lib/gen/auth/v1/auth_pb', () => ({
  AuthService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useSignIn,
  useSignOut,
  useForgotPassword,
  useMFAChallenge,
  useCurrentUser
} from '@/hooks/useAuth'

const mockUseSWR = useSWR as jest.Mock

const mockCreateServiceClient = createServiceClient as jest.Mock

describe('useSignIn', () => {
  it('returns a function that calls client.signIn', () => {
    const mockSignIn = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({ signIn: mockSignIn })

    const { result } = renderHook(() => useSignIn())
    result.current('a@b.com', 'pass', true, '/home')
    expect(mockSignIn).toHaveBeenCalledWith({
      email: 'a@b.com',
      password: 'pass',
      rememberMe: true,
      redirect: '/home'
    })
  })
})

describe('useSignOut', () => {
  it('returns a function that calls client.signOut', () => {
    const mockSignOut = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({ signOut: mockSignOut })

    const { result } = renderHook(() => useSignOut())
    result.current()
    expect(mockSignOut).toHaveBeenCalledWith({})
  })
})

describe('useForgotPassword', () => {
  it('returns a function that calls client.forgotPassword', () => {
    const mockForgotPassword = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      forgotPassword: mockForgotPassword
    })

    const { result } = renderHook(() => useForgotPassword())
    result.current('a@b.com')
    expect(mockForgotPassword).toHaveBeenCalledWith({ email: 'a@b.com' })
  })
})

describe('useMFAChallenge', () => {
  it('returns a function that calls client.mFAChallenge', () => {
    const mockMFAChallenge = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      mFAChallenge: mockMFAChallenge
    })

    const { result } = renderHook(() => useMFAChallenge())
    result.current('654321')
    expect(mockMFAChallenge).toHaveBeenCalledWith({ code: '654321' })
  })
})

describe('useCurrentUser', () => {
  beforeEach(() => {
    mockUseSWR.mockReturnValue({ data: undefined, isLoading: false, error: undefined })
  })

  it('uses /auth/current-user as key', () => {
    renderHook(() => useCurrentUser())
    expect(mockUseSWR).toHaveBeenCalledWith('/auth/current-user', expect.any(Function), {
      revalidateOnFocus: false,
      revalidateOnReconnect: false
    })
  })

  it('returns SWR result', () => {
    const mockData = {}
    mockUseSWR.mockReturnValueOnce({ data: mockData, isLoading: false, error: undefined })
    const { result } = renderHook(() => useCurrentUser())
    expect(result.current.data).toEqual(mockData)
  })
})
