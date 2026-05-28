import React from 'react'
import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import UsersTable from '@/components/admin/UsersTable'
import { AppUserSchema } from '@/lib/gen/admin/v1/admin_pb'

const mockSetRole = jest.fn()
const mockSetAppAccess = jest.fn()

jest.mock('@/hooks/useAdmin', () => ({
  useSetRole: jest.fn(() => mockSetRole),
  useSetAppAccess: jest.fn(() => mockSetAppAccess)
}))

describe('UsersTable', () => {
  beforeEach(() => {
    mockSetRole.mockReset()
    mockSetAppAccess.mockReset()
  })

  const mockUsers = [
    create(AppUserSchema, {
      id: '1',
      email: 'admin@example.com',
      role: 'admin',
      appAccess: ['backlog', 'todos', 'recipes']
    }),
    create(AppUserSchema, {
      id: '2',
      email: 'user@example.com',
      role: 'user',
      appAccess: ['todos']
    })
  ]

  it('renders users table', () => {
    render(<UsersTable users={mockUsers} />)
    expect(screen.getByText('admin@example.com')).toBeInTheDocument()
    expect(screen.getByText('user@example.com')).toBeInTheDocument()
  })

  it('renders role select for each user', () => {
    render(<UsersTable users={mockUsers} />)
    const selects = screen.getAllByRole('combobox')
    expect(selects.length).toBeGreaterThanOrEqual(2)
  })

  it('renders app access checkboxes', () => {
    render(<UsersTable users={mockUsers} />)
    const checkboxes = screen.getAllByRole('checkbox')
    expect(checkboxes.length).toBeGreaterThan(0)
  })

  it('renders all app columns', () => {
    render(<UsersTable users={mockUsers} />)
    expect(screen.getByText('backlog')).toBeInTheDocument()
    expect(screen.getByText('todos')).toBeInTheDocument()
    expect(screen.getByText('recipes')).toBeInTheDocument()
    expect(screen.getByText('contacts')).toBeInTheDocument()
    expect(screen.getByText('watchparty')).toBeInTheDocument()
    expect(screen.getByText('icsproxy')).toBeInTheDocument()
  })

  it('calls setRole and onUpdated when role changes', async () => {
    const onUpdated = jest.fn()
    mockSetRole.mockResolvedValue(undefined)
    render(<UsersTable users={mockUsers} onUpdated={onUpdated} />)

    const selects = screen.getAllByRole('combobox')
    fireEvent.change(selects[0], { target: { value: 'admin' } })

    await waitFor(() => {
      expect(mockSetRole).toHaveBeenCalled()
      expect(onUpdated).toHaveBeenCalled()
    })
  })

  it('calls setAppAccess and onUpdated when checkbox changes', async () => {
    const onUpdated = jest.fn()
    mockSetAppAccess.mockResolvedValue(undefined)
    render(<UsersTable users={mockUsers} onUpdated={onUpdated} />)

    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[0])

    await waitFor(() => {
      expect(mockSetAppAccess).toHaveBeenCalled()
      expect(onUpdated).toHaveBeenCalled()
    })
  })
})
