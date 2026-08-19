import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError } from '@connectrpc/connect'
import ResetPasswordPage from '@/app/auth/reset-password/page'

const mockGet = jest.fn()

jest.mock('next/navigation', () => ({
  useSearchParams: () => ({ get: mockGet })
}))

jest.mock('@/hooks/useAuth', () => ({
  useResetPassword: jest.fn()
}))

import { useResetPassword } from '@/hooks/useAuth'

const mockUseResetPassword = jest.mocked(useResetPassword)

beforeEach(() => {
  jest.clearAllMocks()
  mockGet.mockReturnValue(null)
})

describe('ResetPasswordPage', () => {
  it('shows the invalid state when there is no token query param', async () => {
    mockUseResetPassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    expect(screen.getByText('Invalid or expired reset link.')).toBeInTheDocument()
  })

  it('shows the password form when a token is present', async () => {
    mockGet.mockReturnValue('reset-token')
    mockUseResetPassword.mockReturnValue(jest.fn())

    render(<ResetPasswordPage />)

    expect(screen.getByLabelText('New password')).toBeInTheDocument()
  })

  it('rejects mismatched passwords', async () => {
    mockGet.mockReturnValue('reset-token')
    const mockResetPassword = jest.fn()
    mockUseResetPassword.mockReturnValue(mockResetPassword)

    render(<ResetPasswordPage />)

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'different123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Passwords do not match.')
    })
    expect(mockResetPassword).not.toHaveBeenCalled()
  })

  it('rejects a too-short password', async () => {
    mockGet.mockReturnValue('reset-token')
    const mockResetPassword = jest.fn()
    mockUseResetPassword.mockReturnValue(mockResetPassword)

    render(<ResetPasswordPage />)

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'short' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'short' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Password must be at least 8 characters.')
    })
    expect(mockResetPassword).not.toHaveBeenCalled()
  })

  it('resets the password and shows the done state on success', async () => {
    mockGet.mockReturnValue('reset-token')
    const mockResetPassword = jest.fn().mockResolvedValue({})
    mockUseResetPassword.mockReturnValue(mockResetPassword)

    render(<ResetPasswordPage />)

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'password123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(mockResetPassword).toHaveBeenCalledWith('reset-token', 'password123')
      expect(screen.getByText('Your password has been updated successfully.')).toBeInTheDocument()
    })
  })

  it('shows a ConnectError message when resetting the password fails', async () => {
    mockGet.mockReturnValue('reset-token')
    mockUseResetPassword.mockReturnValue(
      jest.fn().mockRejectedValue(new ConnectError('Reset token expired'))
    )

    render(<ResetPasswordPage />)

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'password123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Reset token expired')
    })
  })

  // The token is re-read from searchParams on every render (not just at
  // mount), so if it goes missing between mount and submit — searchParams
  // updating out from under the page — handleSubmit's own guard must still
  // catch it and flip to the invalid state, rather than calling
  // resetPassword with an empty token.
  it('flips to the invalid state if the token disappears before submit', async () => {
    mockGet.mockReturnValue('reset-token')
    const mockResetPassword = jest.fn()
    mockUseResetPassword.mockReturnValue(mockResetPassword)

    render(<ResetPasswordPage />)

    fireEvent.change(screen.getByLabelText('New password'), { target: { value: 'password123' } })
    // Between the field changes above and the submit below, searchParams
    // stops returning a token.
    mockGet.mockReturnValue(null)
    fireEvent.change(screen.getByLabelText('Confirm new password'), {
      target: { value: 'password123' }
    })
    fireEvent.click(screen.getByRole('button', { name: /Update password/ }))

    await waitFor(() => {
      expect(screen.getByText('Invalid or expired reset link.')).toBeInTheDocument()
    })
    expect(mockResetPassword).not.toHaveBeenCalled()
  })

  it('shows a generic error message when resetting the password fails for a non-Connect reason', async () => {
    mockGet.mockReturnValue('reset-token')
    mockUseResetPassword.mockReturnValue(jest.fn().mockRejectedValue(new Error('network down')))

    render(<ResetPasswordPage />)

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
})
