import { renderHook } from '@testing-library/react'

jest.mock('@/lib/client', () => ({
  createServiceClient: jest.fn(() => ({
    signIn: jest.fn(),
    signOut: jest.fn(),
    forgotPassword: jest.fn(),
    mFAEnroll: jest.fn(),
    mFAEnrollVerify: jest.fn(),
    mFAChallenge: jest.fn()
  }))
}))
jest.mock('@/lib/gen/auth/v1/auth_connect', () => ({
  AuthService: {}
}))

import { createServiceClient } from '@/lib/client'
import {
  useSignIn,
  useSignOut,
  useForgotPassword,
  useMFAEnroll,
  useMFAEnrollVerify,
  useMFAChallenge
} from '@/hooks/useAuth'

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

describe('useMFAEnroll', () => {
  it('returns a function that calls client.mFAEnroll', () => {
    const mockMFAEnroll = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({ mFAEnroll: mockMFAEnroll })

    const { result } = renderHook(() => useMFAEnroll())
    result.current()
    expect(mockMFAEnroll).toHaveBeenCalledWith({})
  })
})

describe('useMFAEnrollVerify', () => {
  it('returns a function that calls client.mFAEnrollVerify', () => {
    const mockMFAEnrollVerify = jest.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      mFAEnrollVerify: mockMFAEnrollVerify
    })

    const { result } = renderHook(() => useMFAEnrollVerify())
    result.current('factor-123', '123456')
    expect(mockMFAEnrollVerify).toHaveBeenCalledWith({
      factorId: 'factor-123',
      code: '123456'
    })
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
