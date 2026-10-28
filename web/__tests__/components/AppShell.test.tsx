import React from 'react'
import { render, screen } from '@testing-library/react'

vi.mock('@/hooks/useAuth', () => ({ useCurrentUser: vi.fn() }))

vi.mock('@/components/SWRProvider', () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="swr-provider">{children}</div>
  ),
  useSessionEffects: vi.fn()
}))

vi.mock('@/components/Navbar', () => ({
  __esModule: true,
  default: () => <div data-testid="navbar" />
}))

vi.mock('@/components/Footer', () => ({
  __esModule: true,
  default: () => <div data-testid="footer" />
}))

vi.mock('@/components/offline/OfflineBanner', () => ({
  __esModule: true,
  default: () => <div data-testid="offline-banner" />
}))

vi.mock('@/components/offline/OutboxSync', () => ({
  __esModule: true,
  default: () => <div data-testid="outbox-sync" />
}))

vi.mock('@/components/offline/ServiceWorkerRegistrar', () => ({
  __esModule: true,
  default: () => <div data-testid="sw-registrar" />
}))

vi.mock('@/components/DeployNotification', () => ({
  __esModule: true,
  default: () => <div data-testid="deploy-notification" />
}))

vi.mock('@/components/books/OfflineBooksSync', () => ({
  __esModule: true,
  default: () => <div data-testid="offline-books-sync" />
}))

import { Code, ConnectError } from '@connectrpc/connect'
import AppShell from '@/components/AppShell'
import { usePathname, useRouter, useSearchParams } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'
import { useSessionEffects } from '@/components/SWRProvider'

function signedInAs(user: { userId?: string; role: string; appAccess: string[] } | undefined) {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only data and error are read
  vi.mocked(useCurrentUser).mockReturnValue({ data: user } as never)
}

function failsWith(error: Error) {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- only data and error are read
  vi.mocked(useCurrentUser).mockReturnValue({ data: undefined, error } as never)
}

function renderAt(pathname: string, search = '') {
  vi.mocked(usePathname).mockReturnValue(pathname)
  vi.mocked(useSearchParams).mockReturnValue(new URLSearchParams(search))
  render(
    <AppShell>
      <div data-testid="child">content</div>
    </AppShell>
  )
}

const replace = () => useRouter().replace

describe('AppShell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sends a signed-out visitor to sign-in, then back to the page', () => {
    failsWith(new ConnectError('not signed in', Code.Unauthenticated))
    renderAt('/feeds/stats', 'feed=1')

    expect(replace()).toHaveBeenCalledWith('/auth/sign-in?next=%2Ffeeds%2Fstats%3Ffeed%3D1')
    expect(screen.queryByTestId('child')).not.toBeInTheDocument()
  })

  it('lets a signed-out visitor stay on a public page', () => {
    failsWith(new ConnectError('not signed in', Code.Unauthenticated))
    renderAt('/dashboard/reading/tok')

    expect(replace()).not.toHaveBeenCalled()
    expect(screen.getByTestId('child')).toBeInTheDocument()
  })

  it('keeps the page when the api is unreachable, so offline mode works', () => {
    failsWith(new TypeError('Failed to fetch'))
    renderAt('/feeds')

    expect(replace()).not.toHaveBeenCalled()
    expect(screen.getByTestId('child')).toBeInTheDocument()
  })

  it('renders the app chrome and children', () => {
    signedInAs({ role: 'user', appAccess: [] })
    render(
      <AppShell>
        <div data-testid="child">content</div>
      </AppShell>
    )

    expect(screen.getByTestId('swr-provider')).toBeInTheDocument()
    expect(screen.getByTestId('navbar')).toBeInTheDocument()
    expect(screen.getByTestId('footer')).toBeInTheDocument()
    expect(screen.getByTestId('deploy-notification')).toBeInTheDocument()
    expect(screen.getByTestId('offline-banner')).toBeInTheDocument()
    expect(screen.getByTestId('sw-registrar')).toBeInTheDocument()
    expect(screen.getByTestId('outbox-sync')).toBeInTheDocument()
    expect(screen.getByTestId('child')).toHaveTextContent('content')
  })

  it('keeps the page clear of a landscape notch at every width', () => {
    signedInAs({ role: 'user', appAccess: [] })
    render(<AppShell>{null}</AppShell>)

    const main = screen.getByRole('main')
    expect(main).toHaveClass('pl-[var(--inset-left)]', 'pr-[var(--inset-right)]')
    expect(main.className).not.toMatch(/\bpx-|(sm|lg):px-/)
  })

  it('passes the signed-in user to the session effects', () => {
    signedInAs({ userId: 'u1', role: 'user', appAccess: [] })
    render(<AppShell>{null}</AppShell>)

    expect(useSessionEffects).toHaveBeenCalledWith('u1')
  })

  it.each([
    ['a user with books access', { role: 'user', appAccess: ['books'] }, true],
    ['an admin', { role: 'admin', appAccess: [] }, true],
    ['a user without books access', { role: 'user', appAccess: ['games'] }, false]
  ])('keeps books downloaded for offline only for %s', (_, user, synced) => {
    signedInAs(user)
    render(<AppShell>{null}</AppShell>)

    expect(screen.queryByTestId('offline-books-sync') !== null).toBe(synced)
  })

  it('keeps no books offline when signed out', () => {
    signedInAs(undefined)
    render(<AppShell>{null}</AppShell>)

    expect(screen.queryByTestId('offline-books-sync')).not.toBeInTheDocument()
  })
})
