jest.mock('@/lib/server/client', () => ({
  createServerClient: jest.fn(async () => ({}))
}))

jest.mock('@/lib/server/fetchers', () => ({
  fetchOrNull: jest.fn(async () => null)
}))

import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/components/games/GamesLibrary', () => () => <div data-testid="games-library" />)

import BacklogGamesLibraryPage from '@/app/games/library/page'

describe('BacklogGamesLibraryPage', () => {
  it('renders the Library heading', async () => {
    render(await BacklogGamesLibraryPage())
    expect(screen.getByRole('heading', { name: 'Library' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /games', async () => {
    render(await BacklogGamesLibraryPage())
    expect(screen.getByRole('link', { name: 'Games' })).toHaveAttribute('href', '/games')
  })

  it('renders the GamesLibrary', async () => {
    render(await BacklogGamesLibraryPage())
    expect(screen.getByTestId('games-library')).toBeInTheDocument()
  })
})
