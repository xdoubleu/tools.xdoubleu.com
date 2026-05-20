import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import Navbar from '@/components/Navbar'

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: jest.fn(),
  useSignOut: jest.fn()
}))

import { useCurrentUser, useSignOut } from '@/hooks/useAuth'

const mockUseCurrentUser = useCurrentUser as jest.Mock
const mockUseSignOut = useSignOut as jest.Mock

beforeEach(() => {
  jest.clearAllMocks()
})

describe('Navbar', () => {
  it('renders nothing while loading', () => {
    mockUseCurrentUser.mockReturnValue({ data: undefined, isLoading: true, error: undefined })
    mockUseSignOut.mockReturnValue(jest.fn())

    const { container } = render(<Navbar />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when unauthenticated', () => {
    mockUseCurrentUser.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })
    mockUseSignOut.mockReturnValue(jest.fn())

    const { container } = render(<Navbar />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nav links when authenticated', () => {
    mockUseCurrentUser.mockReturnValue({
      data: { role: 'user' },
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(jest.fn())

    render(<Navbar />)

    expect(screen.getByRole('link', { name: 'tools.xdoubleu.com' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute('href', '/settings')
    expect(screen.getByRole('link', { name: 'Contacts' })).toHaveAttribute('href', '/contacts')
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
  })

  it('shows Admin link for admin users', () => {
    mockUseCurrentUser.mockReturnValue({
      data: { role: 'admin' },
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(jest.fn())

    render(<Navbar />)

    expect(screen.getByRole('link', { name: 'Admin' })).toHaveAttribute('href', '/admin')
  })

  it('hides Admin link for non-admin users', () => {
    mockUseCurrentUser.mockReturnValue({
      data: { role: 'user' },
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(jest.fn())

    render(<Navbar />)

    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
  })

  it('hides Admin link when role is empty string', () => {
    mockUseCurrentUser.mockReturnValue({ data: { role: '' }, isLoading: false, error: undefined })
    mockUseSignOut.mockReturnValue(jest.fn())

    render(<Navbar />)

    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
  })

  it('calls signOut and redirects to / on sign out', async () => {
    mockUseCurrentUser.mockReturnValue({
      data: { role: 'user' },
      isLoading: false,
      error: undefined
    })
    const mockSignOut = jest.fn().mockResolvedValue({})
    mockUseSignOut.mockReturnValue(mockSignOut)

    render(<Navbar />)

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledTimes(1)
    })
  })
})
