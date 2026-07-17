import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConnectError, Code } from '@connectrpc/connect'

const mockUseCurrentUser = jest.fn()
const mockUpdateDisplayName = jest.fn()
const mockUpdatePassword = jest.fn()
const mockMFAEnroll = jest.fn()
const mockMFAEnrollVerify = jest.fn()
const mockMFAUnenroll = jest.fn()
const mockMutate = jest.fn()

jest.mock('swr', () => ({ mutate: (...args: unknown[]) => mockMutate(...args) }))

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: () => mockUseCurrentUser(),
  useUpdatePassword: () => mockUpdatePassword,
  useUpdateDisplayName: () => mockUpdateDisplayName,
  useMFAEnroll: () => mockMFAEnroll,
  useMFAEnrollVerify: () => mockMFAEnrollVerify,
  useMFAUnenroll: () => mockMFAUnenroll
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
})
