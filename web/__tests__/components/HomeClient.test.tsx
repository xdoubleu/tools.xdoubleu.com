import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError } from '@connectrpc/connect'
import HomeClient from '@/components/HomeClient'

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: jest.fn(),
  useSignIn: jest.fn(),
  useMFAEnroll: jest.fn(),
  useMFAEnrollVerify: jest.fn(),
  useMFAChallenge: jest.fn()
}))

import {
  useCurrentUser,
  useSignIn,
  useMFAEnroll,
  useMFAEnrollVerify,
  useMFAChallenge
} from '@/hooks/useAuth'

const mockUseSettings = useCurrentUser as jest.Mock
const mockUseSignIn = useSignIn as jest.Mock
const mockUseMFAEnroll = useMFAEnroll as jest.Mock
const mockUseMFAEnrollVerify = useMFAEnrollVerify as jest.Mock
const mockUseMFAChallenge = useMFAChallenge as jest.Mock

beforeEach(() => {
  jest.clearAllMocks()
})

describe('HomeClient', () => {
  it('renders loading indicator when isLoading is true', () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<HomeClient />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  it('renders all 5 app links when authenticated', async () => {
    mockUseSettings.mockReturnValue({
      data: { role: 'admin', appAccess: [], integrations: {} },
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByText('Backlog')).toBeInTheDocument()
      expect(screen.getByText('Watch Party')).toBeInTheDocument()
      expect(screen.getByText('ICS Proxy')).toBeInTheDocument()
      expect(screen.getByText('Recipes')).toBeInTheDocument()
      expect(screen.getByText('Todos')).toBeInTheDocument()
    })

    expect(screen.getByRole('link', { name: /Backlog/ })).toHaveAttribute('href', '/backlog')
    expect(screen.getByRole('link', { name: /Watch Party/ })).toHaveAttribute('href', '/watchparty')
    expect(screen.getByRole('link', { name: /ICS Proxy/ })).toHaveAttribute('href', '/icsproxy')
    expect(screen.getByRole('link', { name: /Recipes/ })).toHaveAttribute('href', '/recipes')
    expect(screen.getByRole('link', { name: /Todos/ })).toHaveAttribute('href', '/todos')

    expect(screen.queryByRole('textbox', { name: /Email/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: /Password/ })).not.toBeInTheDocument()
  })

  it('renders sign-in form when unauthenticated', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByLabelText('Email')).toBeInTheDocument()
      expect(screen.getByLabelText('Password')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Sign in/ })).toBeInTheDocument()
      expect(screen.getByRole('link', { name: /Forgot password/i })).toBeInTheDocument()
    })
  })

  it('calls signIn with correct args on successful submission', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({})
    mockUseSignIn.mockReturnValue(mockSignIn)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith('test@example.com', 'password123', true, '')
    })
  })

  it('shows ConnectError message on sign-in failure', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const connectError = new ConnectError('Invalid credentials')
    const mockSignIn = jest.fn().mockRejectedValue(connectError)
    mockUseSignIn.mockReturnValue(mockSignIn)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'wrong' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByRole('alert', { hidden: false })).toHaveTextContent('Invalid credentials')
    })
  })

  it('shows "Sign-in failed." for generic errors', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockRejectedValue(new Error('Network error'))
    mockUseSignIn.mockReturnValue(mockSignIn)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Sign-in failed.')
    })
  })

  it('toggles rememberMe checkbox correctly', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({})
    mockUseSignIn.mockReturnValue(mockSignIn)

    render(<HomeClient />)

    const rememberMeCheckbox = screen.getByLabelText('Remember me')
    expect(rememberMeCheckbox).toBeChecked()

    fireEvent.click(rememberMeCheckbox)
    expect(rememberMeCheckbox).not.toBeChecked()

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith('test@example.com', 'password123', false, '')
    })
  })

  it('disables submit button while submitting', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    let resolveSignIn: () => void
    const neverResolvingPromise = new Promise<void>((resolve) => {
      resolveSignIn = resolve
    })
    const mockSignIn = jest.fn().mockReturnValue(neverResolvingPromise)
    mockUseSignIn.mockReturnValue(mockSignIn)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    expect(submitButton).not.toBeDisabled()

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(submitButton).toHaveTextContent('Signing in...')
      expect(submitButton).toBeDisabled()
    })

    resolveSignIn!()
  })

  it('renders app descriptions', async () => {
    mockUseSettings.mockReturnValue({
      data: { role: 'admin', appAccess: [], integrations: {} },
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByText('Goals and backlog tracker')).toBeInTheDocument()
      expect(screen.getByText('WebRTC screen sharing')).toBeInTheDocument()
      expect(screen.getByText('Calendar feed filtering')).toBeInTheDocument()
      expect(screen.getByText('Recipe management')).toBeInTheDocument()
      expect(screen.getByText('Task management')).toBeInTheDocument()
    })
  })

  it('renders only granted apps for non-admin user', async () => {
    mockUseSettings.mockReturnValue({
      data: { role: 'user', appAccess: ['backlog', 'todos'], integrations: {} },
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByText('Backlog')).toBeInTheDocument()
      expect(screen.getByText('Todos')).toBeInTheDocument()
    })

    expect(screen.queryByText('Watch Party')).not.toBeInTheDocument()
    expect(screen.queryByText('ICS Proxy')).not.toBeInTheDocument()
    expect(screen.queryByText('Recipes')).not.toBeInTheDocument()
  })

  it('renders all apps for admin user', async () => {
    mockUseSettings.mockReturnValue({
      data: { role: 'admin', appAccess: [], integrations: {} },
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByText('Backlog')).toBeInTheDocument()
      expect(screen.getByText('Watch Party')).toBeInTheDocument()
      expect(screen.getByText('ICS Proxy')).toBeInTheDocument()
      expect(screen.getByText('Recipes')).toBeInTheDocument()
      expect(screen.getByText('Todos')).toBeInTheDocument()
    })
  })

  it('renders no apps when appAccess is empty for non-admin', async () => {
    mockUseSettings.mockReturnValue({
      data: { role: 'user', appAccess: [], integrations: {} },
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.queryByText('Backlog')).not.toBeInTheDocument()
      expect(screen.queryByText('Watch Party')).not.toBeInTheDocument()
      expect(screen.queryByText('ICS Proxy')).not.toBeInTheDocument()
      expect(screen.queryByText('Recipes')).not.toBeInTheDocument()
      expect(screen.queryByText('Todos')).not.toBeInTheDocument()
    })
  })

  it('shows MFA enrollment UI when enrollMfa is true', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: true })
    const mockMFAEnroll = jest.fn().mockResolvedValue({
      factorId: 'factor-123',
      qrSvg: '<svg>test</svg>',
      secret: 'JBSWY3DPEBLW64TMMQ'
    })
    const mockMFAEnrollVerify = jest.fn()

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(mockMFAEnroll)
    mockUseMFAEnrollVerify.mockReturnValue(mockMFAEnrollVerify)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Set up two-factor authentication')).toBeInTheDocument()
      expect(screen.getByText(/JBSWY3DPEBLW64TMMQ/)).toBeInTheDocument()
      expect(screen.getByLabelText('Authenticator code')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /Verify/ })).toBeInTheDocument()
    })
  })

  it('does not call mFAEnroll when enrollMfa is false', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const mockMFAChallenge = jest.fn()

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(jest.fn())
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
      expect(screen.queryByText('Set up two-factor authentication')).not.toBeInTheDocument()
    })
  })

  it('shows MFA challenge UI when needsMfa is true', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const mockMFAChallenge = jest.fn()

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
      expect(screen.getByText(/Enter the code from your authenticator app/)).toBeInTheDocument()
    })
  })

  it('calls mFAEnrollVerify on successful MFA enrollment verify', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: true })
    const mockMFAEnroll = jest.fn().mockResolvedValue({
      factorId: 'factor-123',
      qrSvg: '<svg>test</svg>',
      secret: 'JBSWY3DPEBLW64TMMQ'
    })
    const mockMFAEnrollVerify = jest.fn().mockResolvedValue({})

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(mockMFAEnroll)
    mockUseMFAEnrollVerify.mockReturnValue(mockMFAEnrollVerify)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Set up two-factor authentication')).toBeInTheDocument()
    })

    const mfaInput = screen.getByLabelText('Authenticator code')
    const verifyButton = screen.getByRole('button', { name: /Verify/ })

    fireEvent.change(mfaInput, { target: { value: '123456' } })
    fireEvent.click(verifyButton)

    await waitFor(() => {
      expect(mockMFAEnrollVerify).toHaveBeenCalledWith('factor-123', '123456')
    })
  })

  it('shows ConnectError on failed MFA enrollment verify', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: true })
    const mockMFAEnroll = jest.fn().mockResolvedValue({
      factorId: 'factor-123',
      qrSvg: '<svg>test</svg>',
      secret: 'JBSWY3DPEBLW64TMMQ'
    })
    const connectError = new ConnectError('Invalid code')
    const mockMFAEnrollVerify = jest.fn().mockRejectedValue(connectError)

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(mockMFAEnroll)
    mockUseMFAEnrollVerify.mockReturnValue(mockMFAEnrollVerify)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Set up two-factor authentication')).toBeInTheDocument()
    })

    const mfaInput = screen.getByLabelText('Authenticator code')
    const verifyButton = screen.getByRole('button', { name: /Verify/ })

    fireEvent.change(mfaInput, { target: { value: '123456' } })
    fireEvent.click(verifyButton)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Invalid code')
    })
  })

  it('shows generic error message on MFA enrollment verify failure', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: true })
    const mockMFAEnroll = jest.fn().mockResolvedValue({
      factorId: 'factor-123',
      qrSvg: '<svg>test</svg>',
      secret: 'JBSWY3DPEBLW64TMMQ'
    })
    const mockMFAEnrollVerify = jest.fn().mockRejectedValue(new Error('Network error'))

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(mockMFAEnroll)
    mockUseMFAEnrollVerify.mockReturnValue(mockMFAEnrollVerify)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Set up two-factor authentication')).toBeInTheDocument()
    })

    const mfaInput = screen.getByLabelText('Authenticator code')
    const verifyButton = screen.getByRole('button', { name: /Verify/ })

    fireEvent.change(mfaInput, { target: { value: '123456' } })
    fireEvent.click(verifyButton)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Verification failed.')
    })
  })

  it('calls mFAChallenge on successful MFA challenge', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const mockMFAChallenge = jest.fn().mockResolvedValue({})

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    })

    const mfaChallengeInput = screen.getByLabelText('Authenticator code')
    const verifyButton = screen.getByRole('button', { name: /Verify/ })

    fireEvent.change(mfaChallengeInput, { target: { value: '654321' } })
    fireEvent.click(verifyButton)

    await waitFor(() => {
      expect(mockMFAChallenge).toHaveBeenCalledWith('654321')
    })
  })

  it('auto-submits MFA challenge when code reaches 6 digits', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const mockMFAChallenge = jest.fn().mockResolvedValue({})

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    })

    const mfaChallengeInput = screen.getByLabelText('Authenticator code')
    fireEvent.change(mfaChallengeInput, { target: { value: '654321' } })

    await waitFor(() => {
      expect(mockMFAChallenge).toHaveBeenCalledWith('654321')
    })
  })

  it('shows ConnectError on failed MFA challenge', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const connectError = new ConnectError('Invalid code')
    const mockMFAChallenge = jest.fn().mockRejectedValue(connectError)

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    })

    const mfaChallengeInput = screen.getByLabelText('Authenticator code')
    const verifyButton = screen.getByRole('button', { name: /Verify/ })

    fireEvent.change(mfaChallengeInput, { target: { value: '654321' } })
    fireEvent.click(verifyButton)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Invalid code')
    })
  })

  it('shows generic error message on MFA challenge failure', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const mockMFAChallenge = jest.fn().mockRejectedValue(new Error('Network error'))

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    })

    const mfaChallengeInput = screen.getByLabelText('Authenticator code')
    const verifyButton = screen.getByRole('button', { name: /Verify/ })

    fireEvent.change(mfaChallengeInput, { target: { value: '654321' } })
    fireEvent.click(verifyButton)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Challenge failed.')
    })
  })

  it('disables MFA verify button while submitting', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: true })

    let resolveEnrollVerify: () => void
    const neverResolvingPromise = new Promise<void>((resolve) => {
      resolveEnrollVerify = resolve
    })

    const mockMFAEnroll = jest.fn().mockResolvedValue({
      factorId: 'factor-123',
      qrSvg: '<svg>test</svg>',
      secret: 'JBSWY3DPEBLW64TMMQ'
    })
    const mockMFAEnrollVerify = jest.fn().mockReturnValue(neverResolvingPromise)

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(mockMFAEnroll)
    mockUseMFAEnrollVerify.mockReturnValue(mockMFAEnrollVerify)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Set up two-factor authentication')).toBeInTheDocument()
    })

    const mfaInput = screen.getByLabelText('Authenticator code')
    const verifyButton = screen.getByRole('button', { name: /Verify/ })

    expect(verifyButton).not.toBeDisabled()

    fireEvent.change(mfaInput, { target: { value: '123456' } })
    fireEvent.click(verifyButton)

    await waitFor(() => {
      expect(verifyButton).toHaveTextContent('Verifying...')
      expect(verifyButton).toBeDisabled()
    })

    resolveEnrollVerify!()
  })

  it('shows error during MFA enrollment initialization', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: true })
    const enrollError = new ConnectError('Failed to initialize enrollment')
    const mockMFAEnroll = jest.fn().mockRejectedValue(enrollError)

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(mockMFAEnroll)

    render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Failed to initialize enrollment')
      expect(screen.getByText('Sign In')).toBeInTheDocument()
    })
  })

  it('auth error after mfa-challenge does not revert to sign-in form', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const mockMFAChallenge = jest.fn().mockResolvedValue({})

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    const { rerender } = render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    })

    // Update mockUseSettings to return error again
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    rerender(<HomeClient />)

    // Assert that the MFA form is still visible, not the sign-in form
    expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Sign in/ })).not.toBeInTheDocument()
  })

  it('auth error after mfa-enroll does not revert to sign-in form', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: true })
    const mockMFAEnroll = jest.fn().mockResolvedValue({
      factorId: 'f1',
      qrSvg: '<svg/>',
      secret: 'secret'
    })

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAEnroll.mockReturnValue(mockMFAEnroll)

    const { rerender } = render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Set up two-factor authentication')).toBeInTheDocument()
    })

    // Update mockUseSettings to return error again
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    rerender(<HomeClient />)

    // Assert that the MFA enrollment form is still visible, not the sign-in form
    expect(screen.getByText('Set up two-factor authentication')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Sign in/ })).not.toBeInTheDocument()
  })

  it('auth data after mfa-challenge transitions to authenticated', async () => {
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true, enrollMfa: false })
    const mockMFAChallenge = jest.fn().mockResolvedValue({})

    mockUseSignIn.mockReturnValue(mockSignIn)
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)

    const { rerender } = render(<HomeClient />)

    const emailInput = screen.getByLabelText('Email')
    const passwordInput = screen.getByLabelText('Password')
    const submitButton = screen.getByRole('button', { name: /Sign in/ })

    fireEvent.change(emailInput, { target: { value: 'test@example.com' } })
    fireEvent.change(passwordInput, { target: { value: 'password123' } })
    fireEvent.click(submitButton)

    await waitFor(() => {
      expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    })

    // Update mockUseSettings to return user data
    const mockSettings = { role: 'admin', appAccess: [], integrations: {} }
    mockUseSettings.mockReturnValue({
      data: mockSettings,
      isLoading: false,
      error: undefined
    })

    rerender(<HomeClient />)

    // Assert that the app grid is visible, not the MFA form
    expect(screen.getByText('Backlog')).toBeInTheDocument()
    expect(screen.queryByText('Two-factor authentication')).not.toBeInTheDocument()
  })
})
