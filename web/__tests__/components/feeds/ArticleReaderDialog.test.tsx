import { render, screen, fireEvent } from '@testing-library/react'
import { forwardRef, useImperativeHandle } from 'react'
import { create } from '@bufbuild/protobuf'
import { ItemSchema } from '@/lib/gen/feeds/v1/feeds_pb'

const markRead = jest.fn()
const updateItem = jest.fn()

jest.mock('@/components/feeds/FeedBookmarkButton', () => () => (
  <div data-testid="bookmark-button" />
))
jest.mock('@/components/feeds/FeedItemMarkReadButton', () => ({
  __esModule: true,
  default: forwardRef(function MockMarkReadButton(_props: unknown, ref) {
    useImperativeHandle(ref, () => ({ markRead }))
    return <div data-testid="mark-read-button" />
  })
}))
jest.mock('@/hooks/useFeeds', () => ({
  useUpdateItem: () => updateItem
}))

import ArticleReaderDialog from '@/components/feeds/ArticleReaderDialog'

describe('ArticleReaderDialog', () => {
  beforeEach(() => {
    markRead.mockReset()
    updateItem.mockReset()
    updateItem.mockResolvedValue({})
  })

  it('auto-marks the item read once scrolled to the end of the content', () => {
    const item = create(ItemSchema, {
      id: 'item-1',
      title: 'Long Article',
      contentHtml: '<p>Body</p>'
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

    const content = screen.getByText('Body').parentElement!.parentElement!
    Object.defineProperty(content, 'scrollHeight', { value: 1000, configurable: true })
    Object.defineProperty(content, 'clientHeight', { value: 300, configurable: true })

    fireEvent.scroll(content, { target: { scrollTop: 400 } })
    expect(markRead).not.toHaveBeenCalled()

    fireEvent.scroll(content, { target: { scrollTop: 690 } })
    expect(markRead).toHaveBeenCalledTimes(1)
  })

  it('auto-marks read on mount when the content already fits without scrolling', () => {
    const clientHeight = jest
      .spyOn(HTMLElement.prototype, 'clientHeight', 'get')
      .mockReturnValue(300)
    const scrollHeight = jest
      .spyOn(HTMLElement.prototype, 'scrollHeight', 'get')
      .mockReturnValue(200)

    const item = create(ItemSchema, { id: 'item-1', title: 'Short', contentHtml: '<p>Body</p>' })
    render(
      <ArticleReaderDialog
        item={item}
        open
        onOpenChange={jest.fn()}
        onMarkRead={jest.fn()}
        onSettled={jest.fn()}
      />
    )

    expect(markRead).toHaveBeenCalledTimes(1)

    clientHeight.mockRestore()
    scrollHeight.mockRestore()
  })

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
    expect(screen.getByTestId('bookmark-button')).toBeInTheDocument()
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

  it('persists the furthest scroll percentage after the debounce window', () => {
    jest.useFakeTimers()
    try {
      const item = create(ItemSchema, {
        id: 'item-1',
        title: 'Long Article',
        contentHtml: '<p>Body</p>'
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

      const content = screen.getByText('Body').parentElement!.parentElement!
      Object.defineProperty(content, 'scrollHeight', { value: 1000, configurable: true })
      Object.defineProperty(content, 'clientHeight', { value: 300, configurable: true })

      fireEvent.scroll(content, { target: { scrollTop: 200 } })
      expect(updateItem).not.toHaveBeenCalled()

      jest.advanceTimersByTime(1000)
      expect(updateItem).toHaveBeenCalledWith('item-1', { readProgressPct: 50 })
    } finally {
      jest.useRealTimers()
    }
  })

  it('flushes read progress on unmount without waiting for the debounce', () => {
    jest.useFakeTimers()
    try {
      const item = create(ItemSchema, {
        id: 'item-1',
        title: 'Long Article',
        contentHtml: '<p>Body</p>'
      })
      const { unmount } = render(
        <ArticleReaderDialog
          item={item}
          open
          onOpenChange={jest.fn()}
          onMarkRead={jest.fn()}
          onSettled={jest.fn()}
        />
      )

      const content = screen.getByText('Body').parentElement!.parentElement!
      Object.defineProperty(content, 'scrollHeight', { value: 1000, configurable: true })
      Object.defineProperty(content, 'clientHeight', { value: 300, configurable: true })

      fireEvent.scroll(content, { target: { scrollTop: 200 } })
      expect(updateItem).not.toHaveBeenCalled()

      unmount()
      expect(updateItem).toHaveBeenCalledWith('item-1', { readProgressPct: 50 })
    } finally {
      jest.useRealTimers()
    }
  })

  it('opens an enlarged pop-up when an article image is clicked', () => {
    const item = create(ItemSchema, {
      id: 'item-1',
      title: 'Article',
      contentHtml:
        '<p>Body</p><a href="https://example.com"><img src="https://img.test/a.png" /></a>'
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

    expect(screen.queryByRole('button', { name: 'Close image' })).not.toBeInTheDocument()

    fireEvent.click(document.querySelector('img')!)

    const zoomed = screen.getByRole('button', { name: 'Close image' })
    expect(zoomed.querySelector('img')).toHaveAttribute('src', 'https://img.test/a.png')

    fireEvent.click(zoomed)
    expect(screen.queryByRole('button', { name: 'Close image' })).not.toBeInTheDocument()
  })

  it('ignores clicks on non-image article content', () => {
    const item = create(ItemSchema, { id: 'item-1', title: 'Article', contentHtml: '<p>Body</p>' })
    render(
      <ArticleReaderDialog
        item={item}
        open
        onOpenChange={jest.fn()}
        onMarkRead={jest.fn()}
        onSettled={jest.fn()}
      />
    )

    fireEvent.click(screen.getByText('Body'))
    expect(screen.queryByRole('button', { name: 'Close image' })).not.toBeInTheDocument()
  })
})
