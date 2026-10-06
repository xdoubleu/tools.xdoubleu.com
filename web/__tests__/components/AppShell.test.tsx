import React from 'react'
import { render, screen } from '@testing-library/react'

const fetchOrNull = jest.fn()

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({
    getCurrentUser: jest.fn()
  }))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: (fn: () => Promise<unknown>) => fetchOrNull(fn)
}))

jest.mock('@/components/SWRProvider', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="swr-provider">{children}</div>
  )
}))

jest.mock('@/components/Navbar', () => ({
  __esModule: true,
  default: () => <div data-testid="navbar" />
}))

jest.mock('@/components/Footer', () => ({
  __esModule: true,
  default: () => <div data-testid="footer" />
}))

jest.mock('@/components/offline/OfflineBanner', () => ({
  __esModule: true,
  default: () => <div data-testid="offline-banner" />
}))

jest.mock('@/components/offline/OutboxSync', () => ({
  __esModule: true,
  default: () => <div data-testid="outbox-sync" />
}))

jest.mock('@/components/offline/ServiceWorkerRegistrar', () => ({
  __esModule: true,
  default: () => <div data-testid="sw-registrar" />
}))

jest.mock('@/components/DeployNotification', () => ({
  __esModule: true,
  default: () => <div data-testid="deploy-notification" />
}))

jest.mock('@/components/books/OfflineBooksSync', () => ({
  __esModule: true,
  default: () => <div data-testid="offline-books-sync" />
}))

import AppShell from '@/components/AppShell'

describe('AppShell', () => {
  it('renders the app chrome and children once the current-user fetch resolves', async () => {
    fetchOrNull.mockResolvedValue({ role: 'user', appAccess: [] })
    render(await AppShell({ children: <div data-testid="child">content</div> }))

    expect(screen.getByTestId('swr-provider')).toBeInTheDocument()
    expect(screen.getByTestId('navbar')).toBeInTheDocument()
    expect(screen.getByTestId('footer')).toBeInTheDocument()
    expect(screen.getByTestId('deploy-notification')).toBeInTheDocument()
    expect(screen.getByTestId('offline-banner')).toBeInTheDocument()
    expect(screen.getByTestId('sw-registrar')).toBeInTheDocument()
    expect(screen.getByTestId('outbox-sync')).toBeInTheDocument()
    expect(screen.getByTestId('child')).toHaveTextContent('content')
  })

  it('renders when the server fetch returns null', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await AppShell({ children: <div data-testid="child" /> }))

    expect(screen.getByTestId('child')).toBeInTheDocument()
  })

  it.each([
    ['a user with books access', { role: 'user', appAccess: ['books'] }, true],
    ['an admin', { role: 'admin', appAccess: [] }, true],
    ['a user without books access', { role: 'user', appAccess: ['games'] }, false]
  ])('keeps books downloaded for offline only for %s', async (_, user, synced) => {
    fetchOrNull.mockResolvedValue(user)
    render(await AppShell({ children: null }))

    expect(screen.queryByTestId('offline-books-sync') !== null).toBe(synced)
  })

  it('keeps no books offline when signed out', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await AppShell({ children: null }))

    expect(screen.queryByTestId('offline-books-sync')).not.toBeInTheDocument()
  })
})
