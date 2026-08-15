import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/dashboard/GamesDashboard', () => () => (
  <div data-testid="games-dashboard" />
))

jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

import GamesDashboardPage from '@/app/dashboard/games/page'

describe('GamesDashboardPage', () => {
  it('renders the Games heading', async () => {
    render(await GamesDashboardPage())
    expect(screen.getByRole('heading', { name: 'Games' })).toBeInTheDocument()
  })

  it('renders a settings link pointing to /games/settings', async () => {
    render(await GamesDashboardPage())
    const link = screen.getByRole('link', { name: /settings/i })
    expect(link).toHaveAttribute('href', '/games/settings')
  })

  it('renders the GamesDashboard', async () => {
    render(await GamesDashboardPage())
    expect(screen.getByTestId('games-dashboard')).toBeInTheDocument()
  })
})
