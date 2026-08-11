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

jest.mock('@/components/DeployNotification', () => ({
  __esModule: true,
  default: () => <div data-testid="deploy-notification" />
}))

import AppShell from '@/components/AppShell'

describe('AppShell', () => {
  it('renders the app chrome and children once the current-user fetch resolves', async () => {
    fetchOrNull.mockResolvedValue({})
    render(await AppShell({ children: <div data-testid="child">content</div> }))

    expect(screen.getByTestId('swr-provider')).toBeInTheDocument()
    expect(screen.getByTestId('navbar')).toBeInTheDocument()
    expect(screen.getByTestId('footer')).toBeInTheDocument()
    expect(screen.getByTestId('deploy-notification')).toBeInTheDocument()
    expect(screen.getByTestId('child')).toHaveTextContent('content')
  })

  it('renders when the server fetch returns null', async () => {
    fetchOrNull.mockResolvedValue(null)
    render(await AppShell({ children: <div data-testid="child" /> }))

    expect(screen.getByTestId('child')).toBeInTheDocument()
  })
})
