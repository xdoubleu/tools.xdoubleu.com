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
  it('renders the Library heading', () => {
    render(<BacklogGamesLibraryPage />)
    expect(screen.getByRole('heading', { name: 'Library' })).toBeInTheDocument()
  })

  it('renders a breadcrumb link back to /games', () => {
    render(<BacklogGamesLibraryPage />)
    expect(screen.getByRole('link', { name: 'Games' })).toHaveAttribute('href', '/games')
  })

  it('renders the GamesLibrary', () => {
    render(<BacklogGamesLibraryPage />)
    expect(screen.getByTestId('games-library')).toBeInTheDocument()
  })
})
