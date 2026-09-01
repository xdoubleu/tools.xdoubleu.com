import React from 'react'
import { render, screen } from '@testing-library/react'
import ReadingFeedsSummaryCard from '@/components/dashboard/ReadingFeedsSummaryCard'

jest.mock('next/link', () => {
  const Link = ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
  return Object.assign(Link, { useLinkStatus: () => ({ pending: false }) })
})

describe('ReadingFeedsSummaryCard', () => {
  it('renders nothing without a summary', () => {
    const { container } = render(<ReadingFeedsSummaryCard />)
    expect(container).toBeEmptyDOMElement()
  })

  it('links to the given href and lists recent items', () => {
    render(
      <ReadingFeedsSummaryCard
        summary={{
          unreadCount: 3,
          items: [
            { title: 'First post', sourceUrl: 'a', publishedAt: '1' },
            { title: 'Second post', sourceUrl: 'b', publishedAt: '2' }
          ]
        }}
        href="/feeds"
      />
    )
    expect(screen.getByRole('link', { name: /Feeds/ })).toHaveAttribute('href', '/feeds')
    expect(screen.getByText('3 unread')).toBeInTheDocument()
    expect(screen.getByText('First post')).toBeInTheDocument()
    expect(screen.getByText('Second post')).toBeInTheDocument()
  })

  it('shows an empty-state message with no unread items', () => {
    render(<ReadingFeedsSummaryCard summary={{ unreadCount: 0, items: [] }} href="/feeds" />)
    expect(screen.getByText('No unread items.')).toBeInTheDocument()
    expect(screen.getByText('0 unread')).toBeInTheDocument()
  })

  it('renders as a plain card without a link when no href is given', () => {
    render(<ReadingFeedsSummaryCard summary={{ unreadCount: 1, items: [] }} />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.getByText('1 unread')).toBeInTheDocument()
  })
})
