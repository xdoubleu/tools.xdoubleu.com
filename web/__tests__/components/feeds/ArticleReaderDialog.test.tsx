import { render, screen } from '@testing-library/react'
import { create } from '@bufbuild/protobuf'
import { ItemSchema } from '@/lib/gen/feeds/v1/feeds_pb'

jest.mock('@/components/feeds/FeedFavouriteButton', () => () => (
  <div data-testid="favourite-button" />
))
jest.mock('@/components/feeds/FeedItemMarkReadButton', () => () => (
  <div data-testid="mark-read-button" />
))

import ArticleReaderDialog from '@/components/feeds/ArticleReaderDialog'

describe('ArticleReaderDialog', () => {
  it('renders the title and sanitized content', () => {
    const item = create(ItemSchema, {
      id: 'item-1',
      title: 'Hello World',
      contentHtml: '<p>Body <script>alert(1)</script></p>',
      sourceUrl: 'https://example.com/a'
    })
    render(
      <ArticleReaderDialog
        item={item}
        open
        onOpenChange={jest.fn()}
        onMarkRead={jest.fn()}
        onSettled={jest.fn()}
      />
    )

    expect(screen.getByText('Hello World')).toBeInTheDocument()
    expect(screen.getByText('Body')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /View original/ })).toHaveAttribute(
      'href',
      'https://example.com/a'
    )
    expect(screen.getByTestId('favourite-button')).toBeInTheDocument()
    expect(screen.getByTestId('mark-read-button')).toBeInTheDocument()
  })

  it('shows a fallback message when there is no stored content', () => {
    const item = create(ItemSchema, {
      id: 'item-1',
      title: 'No Content',
      contentHtml: '',
      sourceUrl: 'https://example.com/a'
    })
    render(
      <ArticleReaderDialog
        item={item}
        open
        onOpenChange={jest.fn()}
        onMarkRead={jest.fn()}
        onSettled={jest.fn()}
      />
    )

    expect(screen.getByText(/No in-app content stored/)).toBeInTheDocument()
  })

  it('omits the "View original" link when there is no source URL', () => {
    const item = create(ItemSchema, {
      id: 'item-1',
      title: 'No Source',
      contentHtml: '<p>Body</p>',
      sourceUrl: ''
    })
    render(
      <ArticleReaderDialog
        item={item}
        open
        onOpenChange={jest.fn()}
        onMarkRead={jest.fn()}
        onSettled={jest.fn()}
      />
    )

    expect(screen.queryByRole('link', { name: /View original/ })).not.toBeInTheDocument()
  })
})
