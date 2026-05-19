import React from 'react'
import { render, screen } from '@testing-library/react'
import UsersTable from '@/components/admin/UsersTable'
import type { AppUser } from '@/lib/gen/admin/v1/admin_pb'

jest.mock('@/hooks/useAdmin', () => ({
  useSetRole: jest.fn(() => jest.fn()),
  useSetAppAccess: jest.fn(() => jest.fn())
}))

describe('UsersTable', () => {
  const mockUsers: AppUser[] = [
    {
      id: '1',
      email: 'admin@example.com',
      role: 'admin',
      appAccess: ['backlog', 'todos', 'recipes']
    } as unknown as AppUser,
    {
      id: '2',
      email: 'user@example.com',
      role: 'user',
      appAccess: ['todos']
    } as unknown as AppUser
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
})
