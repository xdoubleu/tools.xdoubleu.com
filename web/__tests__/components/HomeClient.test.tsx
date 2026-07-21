import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError } from '@connectrpc/connect'
import HomeClient, { safeNext } from '@/components/HomeClient'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: jest.fn(),
  useSignIn: jest.fn(),
  useMFAChallenge: jest.fn()
}))

import { useCurrentUser, useSignIn, useMFAChallenge } from '@/hooks/useAuth'

const mockUseSettings = jest.mocked(useCurrentUser)
const mockUseSignIn = jest.mocked(useSignIn)
const mockUseMFAChallenge = jest.mocked(useMFAChallenge)

beforeEach(() => {
  jest.clearAllMocks()
})

describe('HomeClient', () => {
  it('renders loading indicator when isLoading is true', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: true,
      error: undefined
    })

    render(<HomeClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  it('renders all app links when authenticated as admin', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'admin', appAccess: [] }),
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Productivity' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Food' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Tools' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Account' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Admin' })).toBeInTheDocument()
      expect(screen.getByText('Games')).toBeInTheDocument()
      expect(screen.getByText('Reading')).toBeInTheDocument()
      expect(screen.getByText('Watch Party')).toBeInTheDocument()
      expect(screen.getByText('ICS Proxy')).toBeInTheDocument()
      expect(screen.getByText('Recipes')).toBeInTheDocument()
      expect(screen.getByText('Meal Plans')).toBeInTheDocument()
      expect(screen.getByText('Shopping List')).toBeInTheDocument()
      expect(screen.getByText('Todos')).toBeInTheDocument()
      expect(screen.getByText('Settings')).toBeInTheDocument()
      expect(screen.getByText('Contacts')).toBeInTheDocument()
    })

    expect(screen.getByRole('link', { name: /Games/ })).toHaveAttribute('href', '/games')
    expect(screen.getByRole('link', { name: /Reading/ })).toHaveAttribute('href', '/reading')
    expect(screen.getByRole('link', { name: /Watch Party/ })).toHaveAttribute('href', '/watchparty')
    expect(screen.getByRole('link', { name: /ICS Proxy/ })).toHaveAttribute('href', '/icsproxy')
    expect(screen.getByRole('link', { name: /Recipes/ })).toHaveAttribute('href', '/recipes/list')
    expect(screen.getByRole('link', { name: /Meal Plans/ })).toHaveAttribute('href', '/mealplans')
    expect(screen.getByRole('link', { name: /Shopping List/ })).toHaveAttribute(
      'href',
      '/shoppinglist'
    )
    expect(screen.getByRole('link', { name: /Todos/ })).toHaveAttribute('href', '/todos')
    expect(screen.getByRole('link', { name: /Settings/ })).toHaveAttribute('href', '/settings')
    expect(screen.getByRole('link', { name: /Contacts/ })).toHaveAttribute('href', '/contacts')
    expect(screen.getByRole('link', { name: /User management/ })).toHaveAttribute(
      'href',
      '/user-management'
    )

    expect(screen.queryByRole('textbox', { name: /Email/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: /Password/ })).not.toBeInTheDocument()
  })

  it('renders sign-in form when unauthenticated', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
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
      expect(submitButton).toHaveTextContent('Signing in…')
      expect(submitButton).toBeDisabled()
    })

    resolveSignIn!()
  })

  it('renders app descriptions', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'admin', appAccess: [] }),
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByText('Steam backlog, progress and distribution.')).toBeInTheDocument()
      expect(screen.getByText('Search, library and reading progress.')).toBeInTheDocument()
      expect(screen.getByText('WebRTC screen sharing')).toBeInTheDocument()
      expect(screen.getByText('Calendar feed filtering')).toBeInTheDocument()
      expect(screen.getByText('Recipe management')).toBeInTheDocument()
      expect(screen.getByText('Task management')).toBeInTheDocument()
      expect(screen.getByText('User preferences')).toBeInTheDocument()
      expect(screen.getByText('Manage contacts')).toBeInTheDocument()
    })
  })

  it('renders only granted apps plus always-visible apps for non-admin user', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, {
        role: 'user',
        appAccess: ['games', 'reading', 'todos']
      }),
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Productivity' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Account' })).toBeInTheDocument()
      expect(screen.getByText('Games')).toBeInTheDocument()
      expect(screen.getByText('Reading')).toBeInTheDocument()
      expect(screen.getByText('Todos')).toBeInTheDocument()
      expect(screen.getByText('Settings')).toBeInTheDocument()
      expect(screen.getByText('Contacts')).toBeInTheDocument()
    })

    expect(screen.queryByText('Watch Party')).not.toBeInTheDocument()
    expect(screen.queryByText('ICS Proxy')).not.toBeInTheDocument()
    expect(screen.queryByText('Recipes')).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Tools' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Food' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Admin' })).not.toBeInTheDocument()
  })

  it('renders all apps for admin user including admin-only and always-visible', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'admin', appAccess: [] }),
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Productivity' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Food' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Tools' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Account' })).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Admin' })).toBeInTheDocument()
      expect(screen.getByText('Games')).toBeInTheDocument()
      expect(screen.getByText('Reading')).toBeInTheDocument()
      expect(screen.getByText('Watch Party')).toBeInTheDocument()
      expect(screen.getByText('ICS Proxy')).toBeInTheDocument()
      expect(screen.getByText('Recipes')).toBeInTheDocument()
      expect(screen.getByText('Meal Plans')).toBeInTheDocument()
      expect(screen.getByText('Shopping List')).toBeInTheDocument()
      expect(screen.getByText('Todos')).toBeInTheDocument()
      expect(screen.getByText('Settings')).toBeInTheDocument()
      expect(screen.getByText('Contacts')).toBeInTheDocument()
    })
  })

  it('shows only always-visible apps when appAccess is empty for non-admin', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
      isLoading: false,
      error: undefined
    })

    render(<HomeClient />)

    await waitFor(() => {
      expect(screen.getByText('Settings')).toBeInTheDocument()
      expect(screen.getByText('Contacts')).toBeInTheDocument()
    })

    expect(screen.queryByText('Games')).not.toBeInTheDocument()
    expect(screen.queryByText('Reading')).not.toBeInTheDocument()
    expect(screen.queryByText('Watch Party')).not.toBeInTheDocument()
    expect(screen.queryByText('ICS Proxy')).not.toBeInTheDocument()
    expect(screen.queryByText('Recipes')).not.toBeInTheDocument()
    expect(screen.queryByText('Todos')).not.toBeInTheDocument()
    expect(screen.queryByText('Admin')).not.toBeInTheDocument()
  })

  describe('safeNext (#446 next-redirect validation)', () => {
    it('returns a same-origin next param unchanged', () => {
      window.history.pushState({}, '', '/?next=%2Freading')
      expect(safeNext()).toBe('/reading')
    })

    it('rejects a protocol-relative //evil.com next param', () => {
      window.history.pushState({}, '', '/?next=%2F%2Fevil.com')
      expect(safeNext()).toBe('/')
    })

    it('rejects the backslash variant browsers normalize to protocol-relative', () => {
      window.history.pushState({}, '', '/?next=%2F%5Cevil.com')
      expect(safeNext()).toBe('/')
    })

    it('defaults to / when next is absent', () => {
      window.history.pushState({}, '', '/')
      expect(safeNext()).toBe('/')
    })
  })

  it('shows MFA challenge UI when needsMfa is true', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true })
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

  it('calls mFAChallenge on successful MFA challenge', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true })
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true })
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true })
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
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true })
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

  it('auth error after mfa-challenge does not revert to sign-in form', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    const mockSignIn = jest.fn().mockResolvedValue({ needsMfa: true })
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

    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseSettings.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })

    rerender(<HomeClient />)

    expect(screen.getByText('Two-factor authentication')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Sign in/ })).not.toBeInTheDocument()
  })
})
