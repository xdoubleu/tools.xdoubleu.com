import { sanitizeArticleHtml } from '@/lib/sanitizeHtml'

describe('sanitizeArticleHtml', () => {
  it('strips script tags', () => {
    expect(sanitizeArticleHtml('<p>Body <script>alert(1)</script></p>')).toBe('<p>Body </p>')
  })

  it('strips style tags so their CSS cannot leak page-wide', () => {
    const html = '<style>body { width: 100% !important; }</style><p>Body</p>'
    expect(sanitizeArticleHtml(html)).toBe('<p>Body</p>')
  })

  it('strips loading="lazy" from images but keeps other attributes (#862)', () => {
    const result = sanitizeArticleHtml('<img src="a.png" loading="lazy" alt="x"/>')
    expect(result).not.toContain('loading')
    expect(result).toContain('src="a.png"')
    expect(result).toContain('alt="x"')
  })

  it('strips nav and header tags (leftover site menus from extraction) (#892)', () => {
    const result = sanitizeArticleHtml('<nav>Menu</nav><header>Site header</header><p>hi</p>')
    expect(result).toBe('<p>hi</p>')
  })
})
