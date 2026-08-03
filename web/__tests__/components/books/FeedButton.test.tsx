import React from 'react'
import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ItemSchema, ListFeedItemsResponseSchema } from '@/lib/gen/feeds/v1/feeds_pb'

jest.mock('next/link', () => {
  return ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  )
})

jest.mock('@/hooks/useFeeds', () => ({
  useFeedItems: jest.fn()
}))

import FeedButton from '@/components/books/FeedButton'
import { useFeedItems } from '@/hooks/useFeeds'

const mockUseFeedItems = jest.mocked(useFeedItems)

function feedItem(id: string, overrides: Partial<{ readAt: string; dismissed: boolean }> = {}) {
  return create(ItemSchema, { id, readAt: '', dismissed: false, ...overrides })
}

describe('FeedButton', () => {
  it('renders without a badge while feed items are loading', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseFeedItems.mockReturnValue({ data: undefined })
    render(<FeedButton />)
    expect(screen.getByRole('link', { name: /feed/i })).toHaveAttribute('href', '/feeds')
    expect(screen.queryByText(/^\d+$/)).not.toBeInTheDocument()
  })

  it('renders a link to /feeds without a badge when there are no unread items', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseFeedItems.mockReturnValue({ data: create(ListFeedItemsResponseSchema, { items: [] }) })
    render(<FeedButton />)
    const link = screen.getByRole('link', { name: /feed/i })
    expect(link).toHaveAttribute('href', '/feeds')
    expect(screen.queryByText(/^\d+$/)).not.toBeInTheDocument()
  })

  it('shows the unread count as a badge', () => {
    // @ts-expect-error -- partial SWRResponse for test purposes
    mockUseFeedItems.mockReturnValue({
      data: create(ListFeedItemsResponseSchema, {
        items: [
          feedItem('1'),
          feedItem('2'),
          feedItem('3', { readAt: '2026-07-17T08:00:00Z' }),
          feedItem('4', { dismissed: true })
        ]
      })
    })
    render(<FeedButton />)
    expect(screen.getByText('2')).toBeInTheDocument()
  })
})
