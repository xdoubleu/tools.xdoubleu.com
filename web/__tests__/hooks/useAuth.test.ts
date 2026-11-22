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
vi.mock('@/lib/account/offlineWrites', () => ({ setTabPinsWrite: { id: 'set-tab-pins' } }))
vi.mock('@/lib/offline/outbox', () => ({
  enqueueWrite: vi.fn(() => Promise.resolve('handle')),
  sendWrite: vi.fn().mockResolvedValue('sent')
}))
vi.mock('@/lib/gen/dashboard/v1/dashboard_pb', () => ({
  DashboardService: {}
}))

import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import {
  useSignIn,
  useSignOut,
  useForgotPassword,
  useExchangeToken,
  useUpdatePassword,
  useResetPassword,
  useRegenerateRecoveryCodes,
  useMFAChallenge,
  useMFAEnroll,
  useMFAEnrollVerify,
  useMFAUnenroll,
  useCurrentUser,
  useSetTabPins,
  useRecordAppOpen,
  usePasskeys,
  useRenamePasskey,
  useRemovePasskey,
  useRegisterPasskey,
  passkeysSupported,
  passkeySignInSupported,
  conditionalPasskeyAvailable,
  usePasskeySignIn
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

describe('useSetTabPins', () => {
  it('queues the write and waits for it to be sent', async () => {
    const { result } = renderHook(() => useSetTabPins())
    await result.current(['books', 'feeds'])
    expect(enqueueWrite).toHaveBeenCalledWith(
      { id: 'set-tab-pins' },
      { tabPins: ['books', 'feeds'] }
    )
    expect(sendWrite).toHaveBeenCalledTimes(1)
  })

  it('throws when the server refuses the write', async () => {
    vi.mocked(sendWrite).mockRejectedValueOnce(new Error('refused'))
    const { result } = renderHook(() => useSetTabPins())
    await expect(result.current(['books'])).rejects.toThrow('refused')
  })
})

describe('useRecordAppOpen', () => {
  it('calls client.recordAppOpen with the app id', () => {
    const recordAppOpen = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ recordAppOpen })
    const { result } = renderHook(() => useRecordAppOpen())
    result.current('recipes')
    expect(recordAppOpen).toHaveBeenCalledWith({ app: 'recipes' })
  })
})

describe('passkey hooks', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('usePasskeys lists through the client', async () => {
    const listPasskeys = vi.fn().mockResolvedValue({ passkeys: [{ id: 'p1' }] })
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ listPasskeys })
    let listed: unknown
    // @ts-expect-error -- mock returns partial SWRResponse for test purposes
    mockUseSWR.mockImplementationOnce((key, fetcher) => {
      // @ts-expect-error -- the mock calls the fetcher SWR would call
      listed = fetcher(key)
      return {}
    })
    renderHook(() => usePasskeys())
    await expect(listed).resolves.toEqual([{ id: 'p1' }])
  })

  it('rename and remove call the client', () => {
    const renamePasskey = vi.fn().mockResolvedValue({})
    const removePasskey = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ renamePasskey, removePasskey })
    renderHook(() => useRenamePasskey()).result.current('p1', 'New')
    renderHook(() => useRemovePasskey()).result.current('p1')
    expect(renamePasskey).toHaveBeenCalledWith({ id: 'p1', name: 'New' })
    expect(removePasskey).toHaveBeenCalledWith({ id: 'p1' })
  })

  it('reports support only when the browser has the JSON helpers', () => {
    expect(passkeysSupported()).toBe(false)
    vi.stubGlobal('PublicKeyCredential', { parseCreationOptionsFromJSON: vi.fn() })
    expect(passkeysSupported()).toBe(true)
    vi.stubGlobal('PublicKeyCredential', {})
    expect(passkeysSupported()).toBe(false)
  })

  describe('useRegisterPasskey', () => {
    class FakeCredential {
      toJSON() {
        return { id: 'cred' }
      }
    }
    const setup = (created: unknown) => {
      const passkeyRegisterBegin = vi.fn().mockResolvedValue({
        optionsJson: JSON.stringify({ challenge: 'abc' })
      })
      const passkeyRegisterFinish = vi.fn().mockResolvedValue({ passkey: { id: 'p1' } })
      // @ts-expect-error -- mock client returns partial shape
      mockCreateServiceClient.mockReturnValue({ passkeyRegisterBegin, passkeyRegisterFinish })
      const parse = vi.fn((o) => ({ parsed: o }))
      const create = vi.fn().mockResolvedValue(created)
      vi.stubGlobal(
        'PublicKeyCredential',
        Object.assign(FakeCredential, { parseCreationOptionsFromJSON: parse })
      )
      vi.stubGlobal('navigator', { credentials: { create } })
      return { passkeyRegisterFinish, create }
    }

    it('runs begin, the authenticator, then finish', async () => {
      const { passkeyRegisterFinish, create } = setup(new FakeCredential())
      const { result } = renderHook(() => useRegisterPasskey())
      await expect(result.current('Laptop')).resolves.toEqual({ id: 'p1' })
      expect(create).toHaveBeenCalledWith({ publicKey: { parsed: { challenge: 'abc' } } })
      expect(passkeyRegisterFinish).toHaveBeenCalledWith({
        name: 'Laptop',
        credentialJson: JSON.stringify({ id: 'cred' })
      })
    })

    it('fails when the browser returns no credential', async () => {
      const { passkeyRegisterFinish } = setup(null)
      const { result } = renderHook(() => useRegisterPasskey())
      await expect(result.current('Laptop')).rejects.toThrow('No passkey was created.')
      expect(passkeyRegisterFinish).not.toHaveBeenCalled()
    })
  })
})

describe('passkey sign-in hooks', () => {
  afterEach(() => vi.unstubAllGlobals())

  class FakeCredential {
    toJSON() {
      return { id: 'cred' }
    }
  }

  it('reports support only when the browser has the request JSON helper', () => {
    expect(passkeySignInSupported()).toBe(false)
    vi.stubGlobal('PublicKeyCredential', { parseRequestOptionsFromJSON: vi.fn() })
    expect(passkeySignInSupported()).toBe(true)
  })

  it('reports conditional mediation from the browser', async () => {
    await expect(conditionalPasskeyAvailable()).resolves.toBe(false)
    vi.stubGlobal('PublicKeyCredential', {
      parseRequestOptionsFromJSON: vi.fn(),
      isConditionalMediationAvailable: vi.fn().mockResolvedValue(true)
    })
    await expect(conditionalPasskeyAvailable()).resolves.toBe(true)
    vi.stubGlobal('PublicKeyCredential', { parseRequestOptionsFromJSON: vi.fn() })
    await expect(conditionalPasskeyAvailable()).resolves.toBe(false)
  })

  const setup = (got: unknown) => {
    const passkeySignInBegin = vi.fn().mockResolvedValue({
      optionsJson: JSON.stringify({ challenge: 'abc' })
    })
    const passkeySignInFinish = vi.fn().mockResolvedValue({})
    // @ts-expect-error -- mock client returns partial shape
    mockCreateServiceClient.mockReturnValue({ passkeySignInBegin, passkeySignInFinish })
    const get = vi.fn().mockResolvedValue(got)
    vi.stubGlobal(
      'PublicKeyCredential',
      Object.assign(FakeCredential, { parseRequestOptionsFromJSON: vi.fn((o) => ({ parsed: o })) })
    )
    vi.stubGlobal('navigator', { credentials: { get } })
    return { passkeySignInFinish, get }
  }

  it('runs begin, the authenticator, then finish', async () => {
    const { passkeySignInFinish, get } = setup(new FakeCredential())
    await renderHook(() => usePasskeySignIn()).result.current(false)
    expect(get).toHaveBeenCalledWith({ publicKey: { parsed: { challenge: 'abc' } } })
    expect(passkeySignInFinish).toHaveBeenCalledWith({
      credentialJson: JSON.stringify({ id: 'cred' }),
      rememberMe: false
    })
  })

  it('asks for conditional mediation when autofilling', async () => {
    const { get } = setup(new FakeCredential())
    const signal = new AbortController().signal
    await renderHook(() => usePasskeySignIn()).result.current(true, { conditional: true, signal })
    expect(get).toHaveBeenCalledWith(expect.objectContaining({ mediation: 'conditional', signal }))
  })

  it('throws when the authenticator returns no passkey', async () => {
    const { passkeySignInFinish } = setup(null)
    await expect(renderHook(() => usePasskeySignIn()).result.current(true)).rejects.toThrow(
      'No passkey was used.'
    )
    expect(passkeySignInFinish).not.toHaveBeenCalled()
  })
})
