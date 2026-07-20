import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError } from '@connectrpc/connect'
import ResetPasswordPage from '@/app/auth/reset-password/page'

jest.mock('@/hooks/useAuth', () => ({
  useExchangeToken: jest.fn(),
  useMFAChallenge: jest.fn(),
  useUpdatePassword: jest.fn()
}))

import { useExchangeToken, useMFAChallenge, useUpdatePassword } from '@/hooks/useAuth'

const mockUseExchangeToken = jest.mocked(useExchangeToken)
const mockUseMFAChallenge = jest.mocked(useMFAChallenge)
const mockUseUpdatePassword = jest.mocked(useUpdatePassword)

function setHash(accessToken: string, refreshToken: string, type = 'recovery') {
  window.location.hash = `access_token=${accessToken}&refresh_token=${refreshToken}&type=${type}`
}

beforeEach(() => {
  jest.clearAllMocks()
  window.location.hash = ''
})

describe('ResetPasswordPage', () => {
  it('shows the invalid state when the hash is missing required params', async () => {
    mockUseExchangeToken.mockReturnValue(jest.fn())
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    mockUseUpdatePassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    await waitFor(() => {
      expect(screen.getByText('Invalid or expired reset link.')).toBeInTheDocument()
    })
  })

  it('shows the invalid state when the token exchange rejects', async () => {
    setHash('expired', 'refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockRejectedValue(new Error('expired')))
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    mockUseUpdatePassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    await waitFor(() => {
      expect(
        screen.getByText('This reset link has expired. Please request a new one.')
      ).toBeInTheDocument()
    })
  })

  it('shows the password form directly when MFA is not required', async () => {
    setHash('access', 'refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: false }))
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    mockUseUpdatePassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    await waitFor(() => {
      expect(screen.getByLabelText('New password')).toBeInTheDocument()
    })
  })

  it('shows an MFA challenge before the password form when needsMfa is true (#447)', async () => {
    setHash('mfa-access', 'mfa-refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: true }))
    const mockMFAChallenge = jest.fn().mockResolvedValue({})
    mockUseMFAChallenge.mockReturnValue(mockMFAChallenge)
    mockUseUpdatePassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    await waitFor(() => {
      expect(screen.getByLabelText('Authenticator code')).toBeInTheDocument()
    })
    expect(screen.queryByLabelText('New password')).not.toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Authenticator code'), {
      target: { value: '654321' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Verify/ }))

    await waitFor(() => {
      expect(mockMFAChallenge).toHaveBeenCalledWith('654321')
      expect(screen.getByLabelText('New password')).toBeInTheDocument()
    })
  })

  it('shows a ConnectError message when the MFA challenge fails', async () => {
    setHash('mfa-access', 'mfa-refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: true }))
    mockUseMFAChallenge.mockReturnValue(
      jest.fn().mockRejectedValue(new ConnectError('Invalid code'))
    )
    mockUseUpdatePassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    await waitFor(() => {
      expect(screen.getByLabelText('Authenticator code')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByLabelText('Authenticator code'), {
      target: { value: '000000' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Verify/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Invalid code')
    })
  })

  it('rejects mismatched passwords', async () => {
    setHash('access', 'refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: false }))
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    const mockUpdatePassword = jest.fn()
    mockUseUpdatePassword.mockReturnValue(mockUpdatePassword)

    render(<ResetPasswordPage />)
    await waitFor(() => expect(screen.getByLabelText('New password')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'different123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Passwords do not match.')
    })
    expect(mockUpdatePassword).not.toHaveBeenCalled()
  })

  it('rejects a too-short password', async () => {
    setHash('access', 'refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: false }))
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    const mockUpdatePassword = jest.fn()
    mockUseUpdatePassword.mockReturnValue(mockUpdatePassword)

    render(<ResetPasswordPage />)
    await waitFor(() => expect(screen.getByLabelText('New password')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'short' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'short' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Password must be at least 8 characters.')
    })
    expect(mockUpdatePassword).not.toHaveBeenCalled()
  })

  it('updates the password and shows the done state on success', async () => {
    setHash('access', 'refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: false }))
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    const mockUpdatePassword = jest.fn().mockResolvedValue({})
    mockUseUpdatePassword.mockReturnValue(mockUpdatePassword)

    render(<ResetPasswordPage />)
    await waitFor(() => expect(screen.getByLabelText('New password')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'password123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(mockUpdatePassword).toHaveBeenCalledWith('password123')
      expect(screen.getByText('Your password has been updated successfully.')).toBeInTheDocument()
    })
  })

  it('shows a ConnectError message when updating the password fails', async () => {
    setHash('access', 'refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: false }))
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    mockUseUpdatePassword.mockReturnValue(
      jest.fn().mockRejectedValue(new ConnectError('Password too weak'))
    )

    render(<ResetPasswordPage />)
    await waitFor(() => expect(screen.getByLabelText('New password')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'password123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Password too weak')
    })
  })

  it('shows a generic error message when updating the password fails for a non-Connect reason', async () => {
    setHash('access', 'refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: false }))
    mockUseMFAChallenge.mockReturnValue(jest.fn())
    mockUseUpdatePassword.mockReturnValue(jest.fn().mockRejectedValue(new Error('network down')))

    render(<ResetPasswordPage />)
    await waitFor(() => expect(screen.getByLabelText('New password')).toBeInTheDocument())

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'password123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(
        'Failed to update password. Please try again.'
      )
    })
  })

  it('shows a generic error message when the MFA challenge fails for a non-Connect reason', async () => {
    setHash('mfa-access', 'mfa-refresh')
    mockUseExchangeToken.mockReturnValue(jest.fn().mockResolvedValue({ needsMfa: true }))
    mockUseMFAChallenge.mockReturnValue(jest.fn().mockRejectedValue(new Error('network down')))
    mockUseUpdatePassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    await waitFor(() => {
      expect(screen.getByLabelText('Authenticator code')).toBeInTheDocument()
    })

    fireEvent.change(screen.getByLabelText('Authenticator code'), {
      target: { value: '000000' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Verify/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Verification failed. Please try again.')
    })
  })
})
