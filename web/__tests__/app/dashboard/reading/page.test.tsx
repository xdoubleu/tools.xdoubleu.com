import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/dashboard/ReadingDashboard', () => () => (
  <div data-testid="reading-dashboard" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

jest.mock('@/components/SWRFallback', () => ({
  __esModule: true,
  default: ({ children }: { children: React.ReactNode }) => <>{children}</>
}))

import ReadingDashboardPage from '@/app/dashboard/reading/page'

describe('ReadingDashboardPage', () => {
  it('renders the Reading heading', async () => {
    render(await ReadingDashboardPage())
    expect(screen.getByRole('heading', { name: 'Reading' })).toBeInTheDocument()
  })

  it('renders a settings link pointing to /books/settings', async () => {
    render(await ReadingDashboardPage())
    const link = screen.getByRole('link', { name: /settings/i })
    expect(link).toHaveAttribute('href', '/books/settings')
  })

  it('renders the ReadingDashboard', async () => {
    render(await ReadingDashboardPage())
    expect(screen.getByTestId('reading-dashboard')).toBeInTheDocument()
  })
})
