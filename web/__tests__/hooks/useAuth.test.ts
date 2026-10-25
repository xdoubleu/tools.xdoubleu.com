import { renderHook } from '@testing-library/react'

vi.mock('swr', () => ({ __esModule: true, default: vi.fn() }))
vi.mock('@/lib/client', () => ({
  createServiceClient: vi.fn(() => ({
    signIn: vi.fn(),
    signOut: vi.fn(),
    forgotPassword: vi.fn(),
    exchangeToken: vi.fn(),
    updatePassword: vi.fn(),
    resetPassword: vi.fn(),
    regenerateRecoveryCodes: vi.fn(),
    mFAChallenge: vi.fn(),
    mFAEnroll: vi.fn(),
    mFAEnrollVerify: vi.fn(),
    mFAUnenroll: vi.fn(),
    getCurrentUser: vi.fn()
  }))
}))
vi.mock('@/lib/gen/auth/v1/auth_pb', () => ({
  AuthService: {}
}))
vi.mock('@/lib/gen/dashboard/v1/dashboard_pb', () => ({
  DashboardService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import {
  useSignIn,
  useSignOut,
  useForgotPassword,
  useExchangeToken,
  useUpdatePassword,
  useResetPassword,
  useRegenerateRecoveryCodes,
  useUpdateDisplayName,
  useMFAChallenge,
  useMFAEnroll,
  useMFAEnrollVerify,
  useMFAUnenroll,
  useCurrentUser
} from '@/hooks/useAuth'

const mockUseSWR = vi.mocked(useSWR)

const mockCreateServiceClient = vi.mocked(createServiceClient)

describe('useSignIn', () => {
  it('returns a function that calls client.signIn', () => {
    const mockSignIn = vi.fn().mockResolvedValue({})
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
    const mockSignOut = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ signOut: mockSignOut })

    const { result } = renderHook(() => useSignOut())
    result.current()
    expect(mockSignOut).toHaveBeenCalledWith({})
  })
})

describe('useForgotPassword', () => {
  it('returns a function that calls client.forgotPassword', () => {
    const mockForgotPassword = vi.fn().mockResolvedValue({})
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
    const mockExchangeToken = vi.fn().mockResolvedValue({})
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
    const mockUpdatePassword = vi.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      updatePassword: mockUpdatePassword
    })

    const { result } = renderHook(() => useUpdatePassword())
    result.current('oldpass123', 'newpass123')
    expect(mockUpdatePassword).toHaveBeenCalledWith({
      currentPassword: 'oldpass123',
      newPassword: 'newpass123'
    })
  })
})

describe('useResetPassword', () => {
  it('returns a function that calls client.resetPassword', () => {
    const mockResetPassword = vi.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      resetPassword: mockResetPassword
    })

    const { result } = renderHook(() => useResetPassword())
    result.current('reset-token', 'newpass123')
    expect(mockResetPassword).toHaveBeenCalledWith({
      token: 'reset-token',
      newPassword: 'newpass123'
    })
  })
})

describe('useRegenerateRecoveryCodes', () => {
  it('returns a function that calls client.regenerateRecoveryCodes', () => {
    const mockRegenerateRecoveryCodes = vi.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      regenerateRecoveryCodes: mockRegenerateRecoveryCodes
    })

    const { result } = renderHook(() => useRegenerateRecoveryCodes())
    result.current('123456')
    expect(mockRegenerateRecoveryCodes).toHaveBeenCalledWith({ code: '123456' })
  })
})

describe('useUpdateDisplayName', () => {
  it('returns a function that calls client.setDisplayName', () => {
    const mockSetDisplayName = vi.fn().mockResolvedValue({})
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
    const mockMFAEnroll = vi.fn().mockResolvedValue({})
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
    const mockMFAEnrollVerify = vi.fn().mockResolvedValue({})
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
    const mockMFAUnenroll = vi.fn().mockResolvedValue({})
    mockCreateServiceClient.mockReturnValue({
      // @ts-expect-error -- mock function assigned to typed client method
      mFAUnenroll: mockMFAUnenroll
    })

    const { result } = renderHook(() => useMFAUnenroll())
    result.current('123456')
    expect(mockMFAUnenroll).toHaveBeenCalledWith({ code: '123456' })
  })
})

describe('useMFAChallenge', () => {
  it('returns a function that calls client.mFAChallenge', () => {
    const mockMFAChallenge = vi.fn().mockResolvedValue({})
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

  it('uses /auth/current-user as key and refetches on reconnect, not on focus', () => {
    renderHook(() => useCurrentUser())
    expect(mockUseSWR).toHaveBeenCalledWith('/auth/current-user', expect.any(Function), {
      revalidateOnFocus: false
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
