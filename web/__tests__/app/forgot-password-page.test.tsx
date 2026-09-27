import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError } from '@connectrpc/connect'
import ForgotPasswordPage from '@/app/auth/forgot-password/page'

jest.mock('@/hooks/useAuth', () => ({
  useForgotPassword: jest.fn()
}))

import { useForgotPassword } from '@/hooks/useAuth'

const mockUseForgotPassword = jest.mocked(useForgotPassword)

function submit(email = 'a@example.com') {
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: email } })
  fireEvent.click(screen.getByRole('button', { name: 'Send reset link' }))
}

beforeEach(() => {
  jest.clearAllMocks()
})

describe('ForgotPasswordPage', () => {
  it('renders the page heading and a labelled email field', () => {
    mockUseForgotPassword.mockReturnValue(jest.fn())

    render(<ForgotPasswordPage />)

    expect(
      screen.getByRole('heading', { level: 1, name: 'Reset your password' })
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveAttribute('autocomplete', 'email')
    expect(screen.getByRole('link', { name: 'Back to sign in' })).toHaveAttribute(
      'href',
      '/auth/sign-in'
    )
  })

  it('shows the confirmation after sending the reset link', async () => {
    const forgotPassword = jest.fn().mockResolvedValue(undefined)
    mockUseForgotPassword.mockReturnValue(forgotPassword)

    render(<ForgotPasswordPage />)
    submit()

    await waitFor(() =>
      expect(screen.getByText(/you will receive a password reset link/)).toBeInTheDocument()
    )
    expect(forgotPassword).toHaveBeenCalledWith('a@example.com')
  })

  it('shows the server message on a ConnectError', async () => {
    mockUseForgotPassword.mockReturnValue(jest.fn().mockRejectedValue(new ConnectError('nope')))

    render(<ForgotPasswordPage />)
    submit()

    expect(await screen.findByRole('alert')).toHaveTextContent('nope')
  })

  it('shows a generic message on other errors', async () => {
    mockUseForgotPassword.mockReturnValue(jest.fn().mockRejectedValue(new Error('x')))

    render(<ForgotPasswordPage />)
    submit()

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Something went wrong. Please try again.'
    )
  })
})
