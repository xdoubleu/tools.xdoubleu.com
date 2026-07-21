import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import UserManagementClient from '@/components/user-management/UserManagementClient'
import { AppUserSchema } from '@/lib/gen/access/v1/access_pb'

const mockUseUsers = jest.fn()
const mockSetRole = jest.fn()
const mockSetAppAccess = jest.fn()
const mockMutate = jest.fn()

jest.mock('@/hooks/useUserManagement', () => ({
  useUsers: () => mockUseUsers(),
  useSetRole: () => mockSetRole,
  useSetAppAccess: () => mockSetAppAccess
}))

jest.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

describe('UserManagementClient', () => {
  beforeEach(() => {
    mockUseUsers.mockReset()
    mockSetRole.mockReset()
    mockSetAppAccess.mockReset()
    mockMutate.mockReset()
  })

  it('shows a loading state', () => {
    mockUseUsers.mockReturnValue({ data: undefined, isLoading: true, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error state', () => {
    mockUseUsers.mockReturnValue({ data: undefined, isLoading: false, error: new Error('nope') })
    render(<UserManagementClient />)
    expect(screen.getByText('Failed to load users.')).toBeInTheDocument()
  })

  it('renders the User Management heading without an Observability button', () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getByRole('heading', { name: 'User Management' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Observability/ })).not.toBeInTheDocument()
  })

  it('shows an empty state when there are no users', () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getByText('No users found.')).toBeInTheDocument()
  })

  it('renders a row per user and calls setRole + setAppAccess on interaction', async () => {
    const users = [
      create(AppUserSchema, { id: '1', email: 'a@b.com', role: 'user', appAccess: ['games'] })
    ]
    mockUseUsers.mockReturnValue({ data: { users }, isLoading: false, error: undefined })
    mockSetRole.mockResolvedValue(undefined)
    mockSetAppAccess.mockResolvedValue(undefined)

    render(<UserManagementClient />)

    expect(screen.getByText('a@b.com')).toBeInTheDocument()

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'admin' } })
    await waitFor(() => expect(mockSetRole).toHaveBeenCalledWith('1', 'admin'))

    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))
    await waitFor(() => expect(mockSetAppAccess).toHaveBeenCalledWith('1', 'games', false))

    expect(mockMutate).toHaveBeenCalledWith('/user-management/users')
  })
})
