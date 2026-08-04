import { sanitizeArticleHtml } from '@/lib/sanitizeHtml'

describe('sanitizeArticleHtml', () => {
  it('strips script tags', () => {
    expect(sanitizeArticleHtml('<p>Body <script>alert(1)</script></p>')).toBe('<p>Body </p>')
  })

  it('strips style tags so their CSS cannot leak page-wide', () => {
    const html = '<style>body { width: 100% !important; }</style><p>Body</p>'
    expect(sanitizeArticleHtml(html)).toBe('<p>Body</p>')
  })
})
