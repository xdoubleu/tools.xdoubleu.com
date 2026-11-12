import { create } from '@bufbuild/protobuf'
import { render, screen } from '@testing-library/react'
import Navbar from '@/components/Navbar'
import { GetCurrentUserResponseSchema } from '@/lib/gen/auth/v1/auth_pb'

vi.mock('@/hooks/useAuth', () => ({
  useCurrentUser: vi.fn(),
  useSignOut: vi.fn(() => vi.fn())
}))

vi.mock('@/hooks/navigation', () => ({
  usePathname: vi.fn()
}))

import { useCurrentUser } from '@/hooks/useAuth'
import { usePathname } from '@/hooks/navigation'

const mockUseCurrentUser = vi.mocked(useCurrentUser)
const mockUsePathname = vi.mocked(usePathname)

function signedIn() {
  // @ts-expect-error -- mock returns partial hook response for test purposes
  mockUseCurrentUser.mockReturnValue({
    data: create(GetCurrentUserResponseSchema, { role: 'user', appAccess: ['feeds'] }),
    isLoading: false,
    error: undefined
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  mockUsePathname.mockReturnValue('/')
})

describe('Navbar', () => {
  it('renders nothing while loading', () => {
    // @ts-expect-error -- mock returns partial hook response for test purposes
    mockUseCurrentUser.mockReturnValue({ data: undefined, isLoading: true, error: undefined })
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
    const { container } = render(<Navbar />)
    expect(container.firstChild).toBeNull()
  })

  it('shows the site name on the home page and the account menu', () => {
    signedIn()
    render(<Navbar />)
    expect(screen.getByRole('link', { name: 'tools.xdoubleu.com' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('button', { name: 'Account menu' })).toBeInTheDocument()
  })

  it('names the current app', () => {
    signedIn()
    mockUsePathname.mockReturnValue('/feeds')
    render(<Navbar />)
    expect(screen.getByRole('link', { name: 'Feeds' })).toBeInTheDocument()
  })

  it('is hidden from lg up, where the sidebar takes over', () => {
    signedIn()
    render(<Navbar />)
    expect(screen.getByRole('banner')).toHaveClass('lg:hidden')
  })

  it('pads the header below the iOS status-bar blur', () => {
    signedIn()
    render(<Navbar />)
    expect(screen.getByRole('banner')).toHaveClass('pt-[var(--inset-top)]')
  })

  it('keeps the header clear of a landscape notch at every width', () => {
    signedIn()
    render(<Navbar />)
    const nav = screen.getByRole('navigation')
    expect(nav).toHaveClass('pl-[var(--inset-left)]', 'sm:pr-[var(--inset-right)]')
    expect(nav.className).not.toMatch(/\b(sm|lg):px-/)
  })

  it.each(['/dashboard/reading/some-token', '/dashboard/games/some-token/library'])(
    'renders nothing on the shared dashboard %s',
    (pathname) => {
      signedIn()
      mockUsePathname.mockReturnValue(pathname)
      const { container } = render(<Navbar />)
      expect(container.firstChild).toBeNull()
    }
  )

  it("renders on the owner's own dashboard", () => {
    signedIn()
    mockUsePathname.mockReturnValue('/dashboard/reading')
    render(<Navbar />)
    expect(screen.getByRole('banner')).toBeInTheDocument()
  })
})
