import React from 'react'
import { render, screen } from '@testing-library/react'

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

import GamesStatCard from '@/components/games/GamesStatCard'

describe('GamesStatCard', () => {
  it('renders a static card when no href is given', () => {
    render(<GamesStatCard label="Total backlog" value={5} />)
    expect(screen.getByText('Total backlog')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  it('renders a clickable link card when an href is given', () => {
    render(<GamesStatCard label="Favourites" value={3} href="/games/library" />)
    const link = screen.getByRole('link')
    expect(link).toHaveAttribute('href', '/games/library')
    expect(link).toHaveTextContent('Favourites')
    expect(link).toHaveTextContent('3')
  })
})
