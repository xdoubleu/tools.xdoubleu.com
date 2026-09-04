import { render, screen } from '@testing-library/react'
import BookArticleReaderDialog from '@/components/books/ArticleReaderDialog'
import { useGetBookContent } from '@/hooks/useBooks'

jest.mock('@/hooks/useBooks', () => ({
  useGetBookContent: jest.fn()
}))

const mockHook = jest.mocked(useGetBookContent)

function mockContent(value: { data?: { html: string }; error?: Error }) {
  // @ts-expect-error -- mock returns partial SWRResponse for test purposes
  mockHook.mockReturnValue(value)
}

const baseProps = {
  bookId: 'book-1',
  title: 'A stored article',
  open: true,
  onOpenChange: jest.fn()
}

describe('BookArticleReaderDialog', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('fetches the book content while open', () => {
    mockContent({ data: { html: '<p>body</p>' } })
    render(<BookArticleReaderDialog {...baseProps} />)
    expect(mockHook).toHaveBeenCalledWith('book-1')
  })

  it('does not fetch while closed', () => {
    mockContent({ data: undefined })
    render(<BookArticleReaderDialog {...baseProps} open={false} />)
    expect(mockHook).toHaveBeenCalledWith(null)
  })

  it('renders the stored article content', () => {
    mockContent({ data: { html: '<p>the article body</p>' } })
    render(<BookArticleReaderDialog {...baseProps} />)
    expect(screen.getByText('the article body')).toBeInTheDocument()
    expect(screen.getByText('A stored article')).toBeInTheDocument()
  })

  it('shows a loading line before the content arrives', () => {
    mockContent({ data: undefined })
    render(<BookArticleReaderDialog {...baseProps} />)
    expect(screen.getByText('Loading…')).toBeInTheDocument()
  })

  it('shows an error line when the fetch fails', () => {
    mockContent({ data: undefined, error: new Error('nope') })
    render(<BookArticleReaderDialog {...baseProps} />)
    expect(screen.getByText('Failed to load article.')).toBeInTheDocument()
    expect(screen.queryByText('Loading…')).not.toBeInTheDocument()
  })

  it('points at the original when there is no stored content', () => {
    mockContent({ data: { html: '' } })
    render(<BookArticleReaderDialog {...baseProps} sourceUrl="https://example.com/a" />)
    expect(screen.getByText(/No in-app content stored for this item/)).toBeInTheDocument()
    expect(screen.getByText(/Use "View original" above instead/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /View original/ })).toHaveAttribute(
      'href',
      'https://example.com/a'
    )
  })

  it('omits the "view original" hint and link when there is no source url', () => {
    mockContent({ data: { html: '' } })
    render(<BookArticleReaderDialog {...baseProps} />)
    expect(screen.getByText(/No in-app content stored for this item/)).toBeInTheDocument()
    expect(screen.queryByText(/Use "View original" above instead/)).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /View original/ })).not.toBeInTheDocument()
  })
})
