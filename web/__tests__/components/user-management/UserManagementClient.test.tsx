import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { Code, ConnectError } from '@connectrpc/connect'
import UserManagementClient from '@/components/user-management/UserManagementClient'
import { AppUserSchema } from '@/lib/gen/access/v1/access_pb'

const mockUseUsers = vi.fn()
const mockSetRole = vi.fn()
const mockSetAppAccess = vi.fn()
const mockSetDisplayName = vi.fn()
const mockCreateUser = vi.fn()
const mockDeactivateUser = vi.fn()
const mockMutate = vi.fn()

vi.mock('@/hooks/useUserManagement', () => ({
  useUsers: () => mockUseUsers(),
  useSetRole: () => mockSetRole,
  useSetAppAccess: () => mockSetAppAccess,
  useSetDisplayName: () => mockSetDisplayName,
  useCreateUser: () => mockCreateUser,
  useDeactivateUser: () => mockDeactivateUser
}))

vi.mock('swr', () => ({
  mutate: (...args: unknown[]) => mockMutate(...args)
}))

describe('UserManagementClient', () => {
  beforeEach(() => {
    mockUseUsers.mockReset()
    mockSetRole.mockReset()
    mockSetAppAccess.mockReset()
    mockSetDisplayName.mockReset()
    mockCreateUser.mockReset()
    mockDeactivateUser.mockReset()
    mockMutate.mockReset()
  })

  it('invites a person and refreshes the list', async () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    mockCreateUser.mockResolvedValue({ user: { email: 'new@example.com' } })
    render(<UserManagementClient />)

    fireEvent.change(screen.getByLabelText('Email of the person to invite'), {
      target: { value: ' new@example.com ' }
    })
    fireEvent.change(screen.getByLabelText('Full name of the person to invite'), {
      target: { value: 'New Person' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }))

    await waitFor(() =>
      expect(mockCreateUser).toHaveBeenCalledWith('new@example.com', 'New Person')
    )
    expect(await screen.findByText('Invite sent to new@example.com.')).toBeInTheDocument()
    expect(mockMutate).toHaveBeenCalled()
  })

  it('says when the person already exists', async () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    mockCreateUser.mockRejectedValue(new ConnectError('dup', Code.AlreadyExists))
    render(<UserManagementClient />)

    fireEvent.change(screen.getByLabelText('Email of the person to invite'), {
      target: { value: 'a@example.com' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }))

    expect(await screen.findByText('A user with this email already exists.')).toBeInTheDocument()
  })

  it('shows the reason for other invite failures', async () => {
    mockUseUsers.mockReturnValue({ data: { users: [] }, isLoading: false, error: undefined })
    mockCreateUser.mockRejectedValueOnce(new ConnectError('bad', Code.InvalidArgument))
    render(<UserManagementClient />)
    fireEvent.change(screen.getByLabelText('Email of the person to invite'), {
      target: { value: 'a@example.com' }
    })
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }))
    expect(await screen.findByText('bad')).toBeInTheDocument()

    mockCreateUser.mockRejectedValueOnce(new ConnectError('x', Code.FailedPrecondition))
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }))
    expect(await screen.findByText('Inviting people is not set up.')).toBeInTheDocument()

    mockCreateUser.mockRejectedValueOnce(new Error('boom'))
    fireEvent.click(screen.getByRole('button', { name: 'Invite' }))
    expect(await screen.findByText('Failed to invite this person.')).toBeInTheDocument()
  })

  it('deactivates a person after confirming', async () => {
    mockUseUsers.mockReturnValue({
      data: { users: [create(AppUserSchema, { id: 'u1', email: 'a@example.com', role: 'user' })] },
      isLoading: false,
      error: undefined
    })
    mockDeactivateUser.mockResolvedValue({})
    render(<UserManagementClient />)

    fireEvent.click(screen.getAllByRole('button', { name: 'Deactivate a@example.com' })[0])
    expect(mockDeactivateUser).not.toHaveBeenCalled()
    fireEvent.click(await screen.findByRole('button', { name: 'Deactivate' }))

    await waitFor(() => expect(mockDeactivateUser).toHaveBeenCalledWith('u1'))
    expect(mockMutate).toHaveBeenCalled()
  })

  it('reports a failed deactivation', async () => {
    mockUseUsers.mockReturnValue({
      data: { users: [create(AppUserSchema, { id: 'u1', email: 'a@example.com', role: 'user' })] },
      isLoading: false,
      error: undefined
    })
    mockDeactivateUser.mockRejectedValue(new Error('boom'))
    render(<UserManagementClient />)

    fireEvent.click(screen.getAllByRole('button', { name: 'Deactivate a@example.com' })[0])
    fireEvent.click(await screen.findByRole('button', { name: 'Deactivate' }))

    expect((await screen.findAllByText('Failed to deactivate.')).length).toBeGreaterThan(0)
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

    const games = screen.getByRole('checkbox', { name: 'games access for a@b.com' })
    expect(games).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'books access for a@b.com' })).not.toBeChecked()

    // A revoke asks first.
    fireEvent.click(games)
    expect(mockSetAppAccess).not.toHaveBeenCalled()
    const dialog = screen.getByRole('dialog', { name: 'Revoke games from a@b.com?' })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Revoke' }))
    await waitFor(() => expect(mockSetAppAccess).toHaveBeenCalledWith('1', 'games', false))

    expect(mockMutate).toHaveBeenCalledWith('/user-management/users')
  })

  it('shows the full name and saves an edit on blur', async () => {
    const users = [
      create(AppUserSchema, {
        id: '1',
        email: 'a@b.com',
        displayName: 'Ada Lovelace',
        role: 'user'
      })
    ]
    mockUseUsers.mockReturnValue({ data: { users }, isLoading: false, error: undefined })
    mockSetDisplayName.mockResolvedValue(undefined)
    render(<UserManagementClient />)

    const [input] = screen.getAllByLabelText('Full name for a@b.com')
    expect(input).toHaveValue('Ada Lovelace')

    fireEvent.change(input, { target: { value: ' Ada King ' } })
    fireEvent.blur(input)
    await waitFor(() => expect(mockSetDisplayName).toHaveBeenCalledWith('1', 'Ada King'))
    expect(mockMutate).toHaveBeenCalledWith('/user-management/users')
  })

  it('restores the name instead of saving a blank or unchanged one', () => {
    const users = [
      create(AppUserSchema, {
        id: '1',
        email: 'a@b.com',
        displayName: 'Ada Lovelace',
        role: 'user'
      })
    ]
    mockUseUsers.mockReturnValue({ data: { users }, isLoading: false, error: undefined })
    render(<UserManagementClient />)

    const [input] = screen.getAllByLabelText('Full name for a@b.com')
    fireEvent.change(input, { target: { value: '  ' } })
    fireEvent.blur(input)
    expect(input).toHaveValue('Ada Lovelace')
    expect(mockSetDisplayName).not.toHaveBeenCalled()
  })

  it('shows an error when saving the name fails', async () => {
    const users = [create(AppUserSchema, { id: '1', email: 'a@b.com', role: 'user' })]
    mockUseUsers.mockReturnValue({ data: { users }, isLoading: false, error: undefined })
    mockSetDisplayName.mockRejectedValue(new Error('boom'))
    render(<UserManagementClient />)

    const [input] = screen.getAllByLabelText('Full name for a@b.com')
    fireEvent.change(input, { target: { value: 'Ada' } })
    fireEvent.blur(input)
    expect((await screen.findAllByText('Failed to save full name.')).length).toBeGreaterThan(0)
  })

  it('grants from the table at once, and drops a revoke that is cancelled', async () => {
    const users = [
      create(AppUserSchema, { id: '1', email: 'a@b.com', role: 'user', appAccess: ['games'] })
    ]
    mockUseUsers.mockReturnValue({ data: { users }, isLoading: false, error: undefined })
    mockSetAppAccess.mockResolvedValue(undefined)
    render(<UserManagementClient />)

    fireEvent.click(screen.getByRole('checkbox', { name: 'books access for a@b.com' }))
    await waitFor(() => expect(mockSetAppAccess).toHaveBeenCalledWith('1', 'books', true))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    mockSetAppAccess.mockClear()
    fireEvent.click(screen.getByRole('checkbox', { name: 'games access for a@b.com' }))
    fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cancel' }))
    expect(mockSetAppAccess).not.toHaveBeenCalled()
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
