import React from 'react'
import { render, screen } from '@testing-library/react'

const mockUseGetBookContent = jest.fn()

jest.mock('@/hooks/useBooks', () => ({
  useGetBookContent: (...args: unknown[]) => mockUseGetBookContent(...args)
}))

jest.mock('@/components/ui/dialog', () => ({
  Dialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-content">{children}</div>
  ),
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogClose: ({ children }: { children: React.ReactNode; 'aria-label'?: string }) => (
    <button>{children}</button>
  )
}))

import ArticleReaderDialog from '@/components/reading/ArticleReaderDialog'

const BOOK_ID = 'book-uuid-1234'
const TITLE = 'How to Read Things'
const SOURCE_URL = 'https://example.com/article'

describe('ArticleReaderDialog', () => {
  beforeEach(() => {
    mockUseGetBookContent.mockReset()
  })

  it('does not render when open is false', () => {
    mockUseGetBookContent.mockReturnValue({ data: null, error: null })
    render(
      <ArticleReaderDialog
        bookId={BOOK_ID}
        title={TITLE}
        sourceUrl={SOURCE_URL}
        open={false}
        onOpenChange={jest.fn()}
      />
    )
    expect(screen.queryByTestId('dialog')).not.toBeInTheDocument()
  })

  it('passes null bookId to useGetBookContent when closed', () => {
    mockUseGetBookContent.mockReturnValue({ data: null, error: null })
    render(
      <ArticleReaderDialog bookId={BOOK_ID} title={TITLE} open={false} onOpenChange={jest.fn()} />
    )
    expect(mockUseGetBookContent).toHaveBeenCalledWith(null)
  })

  it('shows loading state while data is not yet available', () => {
    mockUseGetBookContent.mockReturnValue({ data: null, error: null })
    render(
      <ArticleReaderDialog bookId={BOOK_ID} title={TITLE} open={true} onOpenChange={jest.fn()} />
    )
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows error state when fetch fails', () => {
    mockUseGetBookContent.mockReturnValue({ data: null, error: new Error('boom') })
    render(
      <ArticleReaderDialog bookId={BOOK_ID} title={TITLE} open={true} onOpenChange={jest.fn()} />
    )
    expect(screen.getByText('Failed to load article.')).toBeInTheDocument()
  })

  it('shows an empty-content message when no html was stored', () => {
    mockUseGetBookContent.mockReturnValue({ data: { html: '' }, error: null })
    render(
      <ArticleReaderDialog
        bookId={BOOK_ID}
        title={TITLE}
        sourceUrl={SOURCE_URL}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    expect(screen.getByText(/No in-app content stored/)).toBeInTheDocument()
  })

  it('renders sanitized html content', () => {
    mockUseGetBookContent.mockReturnValue({
      data: { html: '<p>Hello <strong>world</strong></p>' },
      error: null
    })
    render(
      <ArticleReaderDialog bookId={BOOK_ID} title={TITLE} open={true} onOpenChange={jest.fn()} />
    )
    expect(screen.getByText('world')).toBeInTheDocument()
  })

  it('strips script tags and inline event handlers from article html', () => {
    mockUseGetBookContent.mockReturnValue({
      data: {
        html: '<p>Safe</p><script>window.__pwned = true</script><img src=x onerror="window.__pwned = true">'
      },
      error: null
    })
    render(
      <ArticleReaderDialog bookId={BOOK_ID} title={TITLE} open={true} onOpenChange={jest.fn()} />
    )
    expect(document.querySelector('script')).not.toBeInTheDocument()
    const img = document.querySelector('img')
    expect(img).not.toHaveAttribute('onerror')
  })

  it('shows a link to the original source when provided', () => {
    mockUseGetBookContent.mockReturnValue({ data: { html: '<p>Body</p>' }, error: null })
    render(
      <ArticleReaderDialog
        bookId={BOOK_ID}
        title={TITLE}
        sourceUrl={SOURCE_URL}
        open={true}
        onOpenChange={jest.fn()}
      />
    )
    const link = screen.getByRole('link', { name: /View original/ })
    expect(link).toHaveAttribute('href', SOURCE_URL)
  })

  it('shows the article title in the dialog header', () => {
    mockUseGetBookContent.mockReturnValue({ data: { html: '<p>Body</p>' }, error: null })
    render(
      <ArticleReaderDialog bookId={BOOK_ID} title={TITLE} open={true} onOpenChange={jest.fn()} />
    )
    expect(screen.getByText(TITLE)).toBeInTheDocument()
  })
})
