import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import UserManagementClient from '@/components/user-management/UserManagementClient'
import { AppUserSchema } from '@/lib/gen/access/v1/access_pb'

const mockUseUsers = vi.fn()
const mockSetRole = vi.fn()
const mockSetAppAccess = vi.fn()
const mockMutate = vi.fn()

vi.mock('@/hooks/useUserManagement', () => ({
  useUsers: () => mockUseUsers(),
  useSetRole: () => mockSetRole,
  useSetAppAccess: () => mockSetAppAccess
}))

vi.mock('swr', () => ({
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

  it('renders the User management heading without an Observability button', () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getByRole('heading', { name: 'User management' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Observability/ })).not.toBeInTheDocument()
  })

  it('links to the agents page', () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getByRole('link', { name: 'Agents' })).toHaveAttribute(
      'href',
      '/user-management/agents'
    )
  })

  it('shows an empty state when there are no users', () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getAllByText('No users found.').length).toBeGreaterThan(0)
  })

  it('renders a row per user and calls setRole + setAppAccess on interaction', async () => {
    const users = [
      create(AppUserSchema, { id: '1', email: 'a@b.com', role: 'user', appAccess: ['games'] })
    ]
    mockUseUsers.mockReturnValue({ data: { users }, isLoading: false, error: undefined })
    mockSetRole.mockResolvedValue(undefined)
    mockSetAppAccess.mockResolvedValue(undefined)

    render(<UserManagementClient />)

    // Both the mobile card view and the desktop table render in jsdom (CSS
    // `sm:` hiding doesn't remove nodes), so the email appears twice.
    expect(screen.getAllByText('a@b.com').length).toBeGreaterThan(0)

    fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: 'admin' } })
    await waitFor(() => expect(mockSetRole).toHaveBeenCalledWith('1', 'admin'))

    fireEvent.click(screen.getAllByRole('button', { name: 'Revoke' })[0])
    await waitFor(() => expect(mockSetAppAccess).toHaveBeenCalledWith('1', 'games', false))

    expect(mockMutate).toHaveBeenCalledWith('/user-management/users')
  })

  it('toggles app access from the mobile card pills', async () => {
    const users = [
      create(AppUserSchema, { id: '1', email: 'a@b.com', role: 'user', appAccess: ['games'] })
    ]
    mockUseUsers.mockReturnValue({ data: { users }, isLoading: false, error: undefined })
    mockSetAppAccess.mockResolvedValue(undefined)

    render(<UserManagementClient />)

    expect(screen.getByRole('button', { name: 'games' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: 'books' }))
    await waitFor(() => expect(mockSetAppAccess).toHaveBeenCalledWith('1', 'books', true))
  })

  it('offers podcasts access', () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getByRole('columnheader', { name: 'podcasts' })).toBeInTheDocument()
  })

  it('offers taste access', () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    render(<UserManagementClient />)
    expect(screen.getByRole('columnheader', { name: 'taste' })).toBeInTheDocument()
  })
})
