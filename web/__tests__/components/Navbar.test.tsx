import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import Navbar from '@/components/Navbar'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

jest.mock('@/hooks/useAuth', () => ({
  useCurrentUser: jest.fn(),
  useSignOut: jest.fn()
}))

jest.mock('next/navigation', () => ({
  usePathname: jest.fn()
}))

import { useCurrentUser, useSignOut } from '@/hooks/useAuth'
import { usePathname } from 'next/navigation'

const mockUseCurrentUser = jest.mocked(useCurrentUser)
const mockUseSignOut = jest.mocked(useSignOut)
const mockUsePathname = jest.mocked(usePathname)

beforeEach(() => {
  jest.clearAllMocks()
  mockUsePathname.mockReturnValue('/settings')
})

describe('Navbar', () => {
  it('renders nothing while loading', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({ data: undefined, isLoading: true, error: undefined })
    mockUseSignOut.mockReturnValue(jest.fn())

    const { container } = render(<Navbar />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when unauthenticated', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error('401')
    })
    mockUseSignOut.mockReturnValue(jest.fn())

    const { container } = render(<Navbar />)
    expect(container.firstChild).toBeNull()
  })

  it('renders logo and sign out when authenticated', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(jest.fn())

    render(<Navbar />)

    expect(screen.getByRole('link', { name: 'tools.xdoubleu.com' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute('href', '/settings')
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Contacts' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
  })

  it('renders nothing on a public shared profile page', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(jest.fn())
    mockUsePathname.mockReturnValue('/profile/reading/some-token')

    const { container } = render(<Navbar />)
    expect(container.firstChild).toBeNull()
  })

  it('calls signOut and redirects to / on sign out', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
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
