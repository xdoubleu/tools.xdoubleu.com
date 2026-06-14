import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/backlog/GamesDashboard', () => () => <div data-testid="games-dashboard" />)

import BacklogGamesPage from '@/app/backlog/games/page'

describe('BacklogGamesPage', () => {
  it('renders the Games heading', () => {
    render(<BacklogGamesPage />)
    expect(screen.getByRole('heading', { name: 'Games' })).toBeInTheDocument()
  })

  it('renders a settings link pointing to /backlog/settings', () => {
    render(<BacklogGamesPage />)
    const link = screen.getByRole('link', { name: /settings/i })
    expect(link).toHaveAttribute('href', '/backlog/games/settings')
  })

  it('renders the GamesDashboard', () => {
    render(<BacklogGamesPage />)
    expect(screen.getByTestId('games-dashboard')).toBeInTheDocument()
  })
})
