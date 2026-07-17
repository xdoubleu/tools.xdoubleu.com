import { renderHook } from '@testing-library/react'

jest.mock('swr', () => ({ __esModule: true, default: jest.fn() }))
jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    signIn: jest.fn(),
    signOut: jest.fn(),
    forgotPassword: jest.fn(),
    exchangeToken: jest.fn(),
    updatePassword: jest.fn(),
    mFAChallenge: jest.fn(),
    mFAEnroll: jest.fn(),
    mFAEnrollVerify: jest.fn(),
    mFAUnenroll: jest.fn(),
    getCurrentUser: jest.fn()
  }))
}))
jest.mock('@/lib/gen/auth/v1/auth_pb', () => ({
  AuthService: {}
}))
jest.mock('@/lib/gen/profile/v1/profile_pb', () => ({
  ProfileService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useSignIn,
  useSignOut,
  useForgotPassword,
  useExchangeToken,
  useUpdatePassword,
  useUpdateDisplayName,
  useMFAChallenge,
  useMFAEnroll,
  useMFAEnrollVerify,
  useMFAUnenroll,
  useCurrentUser
} from '@/hooks/useAuth'

const mockUseSWR = jest.mocked(useSWR)

const mockCreateServiceClient = jest.mocked(createServiceClient)

describe('useSignIn', () => {
  it('returns a function that calls client.signIn', () => {
    const mockSignIn = jest.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
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
    // @ts-expect-error -- mock client returns partial shape
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
      // @ts-expect-error -- mock function assigned to typed client method
      forgotPassword: mockForgotPassword
    })

    const { result } = renderHook(() => useForgotPassword())
    result.current('a@b.com')
    expect(mockForgotPassword).toHaveBeenCalledWith({ email: 'a@b.com' })
  })
})

describe('useExchangeToken', () => {
  it('returns a function that calls client.exchangeToken', () => {
    const mockExchangeToken = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      exchangeToken: mockExchangeToken
    })

    const { result } = renderHook(() => useExchangeToken())
    result.current('access-tok', 'refresh-tok')
    expect(mockExchangeToken).toHaveBeenCalledWith({
      accessToken: 'access-tok',
      refreshToken: 'refresh-tok'
    })
  })
})

describe('useUpdatePassword', () => {
  it('returns a function that calls client.updatePassword', () => {
    const mockUpdatePassword = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      updatePassword: mockUpdatePassword
    })

    const { result } = renderHook(() => useUpdatePassword())
    result.current('newpass123')
    expect(mockUpdatePassword).toHaveBeenCalledWith({ newPassword: 'newpass123' })
  })
})

describe('useUpdateDisplayName', () => {
  it('returns a function that calls client.setDisplayName', () => {
    const mockSetDisplayName = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      setDisplayName: mockSetDisplayName
    })

    const { result } = renderHook(() => useUpdateDisplayName())
    result.current('Alice')
    expect(mockSetDisplayName).toHaveBeenCalledWith({ displayName: 'Alice' })
  })
})

describe('useMFAEnroll', () => {
  it('returns a function that calls client.mFAEnroll', () => {
    const mockMFAEnroll = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      mFAEnroll: mockMFAEnroll
    })

    const { result } = renderHook(() => useMFAEnroll())
    result.current()
    expect(mockMFAEnroll).toHaveBeenCalledWith({})
  })
})

describe('useMFAEnrollVerify', () => {
  it('returns a function that calls client.mFAEnrollVerify', () => {
    const mockMFAEnrollVerify = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      mFAEnrollVerify: mockMFAEnrollVerify
    })

    const { result } = renderHook(() => useMFAEnrollVerify())
    result.current('factor-id', '123456')
    expect(mockMFAEnrollVerify).toHaveBeenCalledWith({
      factorId: 'factor-id',
      code: '123456'
    })
  })
})

describe('useMFAUnenroll', () => {
  it('returns a function that calls client.mFAUnenroll', () => {
    const mockMFAUnenroll = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      mFAUnenroll: mockMFAUnenroll
    })

    const { result } = renderHook(() => useMFAUnenroll())
    result.current()
    expect(mockMFAUnenroll).toHaveBeenCalledWith({})
  })
})

describe('useMFAChallenge', () => {
  it('returns a function that calls client.mFAChallenge', () => {
    const mockMFAChallenge = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      mFAChallenge: mockMFAChallenge
    })

    const { result } = renderHook(() => useMFAChallenge())
    result.current('654321')
    expect(mockMFAChallenge).toHaveBeenCalledWith({ code: '654321' })
  })
})

describe('useCurrentUser', () => {
  beforeEach(() => {
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
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
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockReturnValueOnce({ data: mockData, isLoading: false, error: undefined })
    const { result } = renderHook(() => useCurrentUser())
    expect(result.current.data).toEqual(mockData)
  })
})
