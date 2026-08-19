import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'

const mockUseCurrentUser = jest.fn()
const mockUpdateDisplayName = jest.fn()
const mockUpdatePassword = jest.fn()
const mockMFAEnroll = jest.fn()
const mockMFAEnrollVerify = jest.fn()
const mockMFAUnenroll = jest.fn()
const mockRegenerateRecoveryCodes = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({ mutate: (...args: unknown[]) => mockMutate(...args) }))

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: () => mockUseCurrentUser(),
  useUpdatePassword: () => mockUpdatePassword,
  useUpdateDisplayName: () => mockUpdateDisplayName,
  useMFAEnroll: () => mockMFAEnroll,
  useMFAEnrollVerify: () => mockMFAEnrollVerify,
  useMFAUnenroll: () => mockMFAUnenroll,
  useRegenerateRecoveryCodes: () => mockRegenerateRecoveryCodes
}))

import SettingsPage from '@/app/settings/page'

describe('SettingsPage', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockUpdateDisplayName.mockResolvedValue({})
    mockUseCurrentUser.mockReturnValue({
      data: { role: 'user', appAccess: [], hasMfa: false, displayName: 'Alice' },
      isLoading: false
    })
  })

  it('prefills the display name field from the current user', () => {
    render(<SettingsPage />)
    expect(screen.getByLabelText('Display name')).toHaveValue('Alice')
  })

  it('saves a new display name', async () => {
    render(<SettingsPage />)

    const input = screen.getByLabelText('Display name')
    fireEvent.change(input, { target: { value: 'Bob' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save display name' }))

    expect(await screen.findByText('Display name updated successfully.')).toBeInTheDocument()
    expect(mockUpdateDisplayName).toHaveBeenCalledWith('Bob')
    expect(mockMutate).toHaveBeenCalledWith('/auth/current-user')
  })

  it('shows an error when saving the display name fails', async () => {
    mockUpdateDisplayName.mockRejectedValue(new Error('boom'))
    render(<SettingsPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Save display name' }))

    await waitFor(() => {
      expect(screen.getByText('Failed to update display name.')).toBeInTheDocument()
    })
  })

  it('shows the server message when saving the display name fails with a ConnectError', async () => {
    mockUpdateDisplayName.mockRejectedValue(
      new ConnectError('set a display name before sharing your profile', Code.FailedPrecondition)
    )
    render(<SettingsPage />)

    fireEvent.click(screen.getByRole('button', { name: 'Save display name' }))

    expect(
      await screen.findByText(/set a display name before sharing your profile/)
    ).toBeInTheDocument()
  })

  it('shows a loading state while the current user is loading', () => {
    mockUseCurrentUser.mockReturnValue({ data: undefined, isLoading: true })
    render(<SettingsPage />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  describe('MFA enrollment recovery codes', () => {
    it('shows the recovery codes dialog after enrollment returns codes', async () => {
      mockMFAEnroll.mockResolvedValue({ qrSvg: '<svg></svg>', secret: 'SECRET', factorId: 'f1' })
      mockMFAEnrollVerify.mockResolvedValue({ recoveryCodes: ['aaaa-bbbb', 'cccc-dddd'] })
      render(<SettingsPage />)

      fireEvent.click(screen.getByRole('button', { name: 'Enable MFA' }))
      await screen.findByLabelText('Authenticator code')

      fireEvent.change(screen.getByLabelText('Authenticator code'), {
        target: { value: '123456' }
      })
      fireEvent.click(screen.getByRole('button', { name: 'Verify & enable' }))

      expect(await screen.findByText('Save your recovery codes')).toBeInTheDocument()
      expect(screen.getByText('aaaa-bbbb')).toBeInTheDocument()
      expect(screen.getByText('cccc-dddd')).toBeInTheDocument()
    })

    it('does not show the dialog when enrollment returns no recovery codes', async () => {
      mockMFAEnroll.mockResolvedValue({ qrSvg: '<svg></svg>', secret: 'SECRET', factorId: 'f1' })
      mockMFAEnrollVerify.mockResolvedValue({ recoveryCodes: [] })
      render(<SettingsPage />)

      fireEvent.click(screen.getByRole('button', { name: 'Enable MFA' }))
      await screen.findByLabelText('Authenticator code')

      fireEvent.change(screen.getByLabelText('Authenticator code'), {
        target: { value: '123456' }
      })
      fireEvent.click(screen.getByRole('button', { name: 'Verify & enable' }))

      await screen.findByText('Two-factor authentication enabled successfully.')
      expect(screen.queryByText('Save your recovery codes')).not.toBeInTheDocument()
    })

    it('requires the confirmation checkbox before the dialog can be dismissed', async () => {
      mockMFAEnroll.mockResolvedValue({ qrSvg: '<svg></svg>', secret: 'SECRET', factorId: 'f1' })
      mockMFAEnrollVerify.mockResolvedValue({ recoveryCodes: ['aaaa-bbbb'] })
      render(<SettingsPage />)

      fireEvent.click(screen.getByRole('button', { name: 'Enable MFA' }))
      await screen.findByLabelText('Authenticator code')
      fireEvent.change(screen.getByLabelText('Authenticator code'), {
        target: { value: '123456' }
      })
      fireEvent.click(screen.getByRole('button', { name: 'Verify & enable' }))
      await screen.findByText('Save your recovery codes')

      const doneButton = screen.getByRole('button', { name: 'Done' })
      expect(doneButton).toBeDisabled()

      fireEvent.click(screen.getByLabelText("I've saved these recovery codes somewhere safe."))
      expect(doneButton).toBeEnabled()

      fireEvent.click(doneButton)
      await waitFor(() => {
        expect(screen.queryByText('Save your recovery codes')).not.toBeInTheDocument()
      })
    })
  })

  describe('regenerating recovery codes', () => {
    beforeEach(() => {
      mockUseCurrentUser.mockReturnValue({
        data: { role: 'user', appAccess: [], hasMfa: true, displayName: 'Alice' },
        isLoading: false
      })
    })

    it('shows the recovery codes dialog on success', async () => {
      mockRegenerateRecoveryCodes.mockResolvedValue({ recoveryCodes: ['new-code-1'] })
      render(<SettingsPage />)

      fireEvent.click(screen.getByRole('button', { name: 'Regenerate recovery codes' }))

      expect(await screen.findByText('Save your recovery codes')).toBeInTheDocument()
      expect(screen.getByText('new-code-1')).toBeInTheDocument()
    })

    it('shows an error when regenerating fails', async () => {
      mockRegenerateRecoveryCodes.mockRejectedValue(new Error('boom'))
      render(<SettingsPage />)

      fireEvent.click(screen.getByRole('button', { name: 'Regenerate recovery codes' }))

      expect(await screen.findByText('Failed to regenerate recovery codes.')).toBeInTheDocument()
    })
  })
})
