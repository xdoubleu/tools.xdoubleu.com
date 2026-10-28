import { create } from '@bufbuild/protobuf'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import Navbar from '@/components/Navbar'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

vi.mock('@/hooks/useAuth', () => ({
  useCurrentUser: vi.fn(),
  useSignOut: vi.fn()
}))

vi.mock('@/hooks/navigation', () => ({
  usePathname: vi.fn()
}))

vi.mock('@/lib/offline/session', () => ({
  clearOfflineData: vi.fn(async () => {})
}))

vi.mock('@/lib/offline/outbox', () => ({
  flushOutbox: vi.fn(async () => {})
}))

import { useCurrentUser, useSignOut } from '@/hooks/useAuth'
import { usePathname } from '@/hooks/navigation'
import { clearOfflineData } from '@/lib/offline/session'
import { flushOutbox } from '@/lib/offline/outbox'

const mockUseCurrentUser = vi.mocked(useCurrentUser)
const mockUseSignOut = vi.mocked(useSignOut)
const mockUsePathname = vi.mocked(usePathname)

beforeEach(() => {
  vi.clearAllMocks()
  mockUsePathname.mockReturnValue('/settings')
})

describe('Navbar', () => {
  it('renders nothing while loading', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({ data: undefined, isLoading: true, error: undefined })
    mockUseSignOut.mockReturnValue(vi.fn())

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
    mockUseSignOut.mockReturnValue(vi.fn())

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
    mockUseSignOut.mockReturnValue(vi.fn())

    render(<Navbar />)

    expect(screen.getByRole('link', { name: 'tools.xdoubleu.com' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveAttribute('href', '/settings')
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Contacts' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
  })

  it('pads the header below the iOS status-bar blur', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(vi.fn())

    render(<Navbar />)

    expect(screen.getByRole('banner')).toHaveClass('pt-[var(--inset-top)]')
  })

  it('keeps the header clear of a landscape notch at every width', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(vi.fn())

    render(<Navbar />)

    const nav = screen.getByRole('navigation')
    expect(nav).toHaveClass('pl-[var(--inset-left)]', 'sm:pr-[var(--inset-right)]')
    expect(nav.className).not.toMatch(/\b(sm|lg):px-/)
  })

  it.each(['/dashboard/reading/some-token', '/dashboard/games/some-token/library'])(
    'renders nothing on the shared dashboard %s',
    (pathname) => {
      // @ts-expect-error -- mock returns partial hook response for test purposes
      mockUseCurrentUser.mockReturnValue({
        data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
        isLoading: false,
        error: undefined
      })
      mockUseSignOut.mockReturnValue(vi.fn())
      mockUsePathname.mockReturnValue(pathname)

      const { container } = render(<Navbar />)
      expect(container.firstChild).toBeNull()
    }
  )

  it("renders on the owner's own dashboard", () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
      isLoading: false,
      error: undefined
    })
    mockUseSignOut.mockReturnValue(vi.fn())
    mockUsePathname.mockReturnValue('/dashboard/reading')

    render(<Navbar />)
    expect(screen.getByRole('banner')).toBeInTheDocument()
  })

  it('sends queued writes, signs out, then clears offline data', async () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({
      data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: [] }),
      isLoading: false,
      error: undefined
    })
    const mockSignOut = vi.fn().mockResolvedValue({})
    mockUseSignOut.mockReturnValue(mockSignOut)

    render(<Navbar />)

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))

    await waitFor(() => {
      expect(clearOfflineData).toHaveBeenCalledTimes(1)
    })
    expect(mockSignOut).toHaveBeenCalledTimes(1)
    expect(vi.mocked(flushOutbox).mock.invocationCallOrder[0]).toBeLessThan(
      mockSignOut.mock.invocationCallOrder[0]
    )
  })
})
