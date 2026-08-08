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

  it('strips a non-semantic share widget list (#906)', () => {
    const html =
      '<p>Real intro.</p>' +
      '<ul role="list">' +
      '<li></li><li></li><li></li><li></li>' +
      '<li><div><p>Share</p><p><a href="#">Copy link</a></p><p>https://example.com/post</p></div></li>' +
      '</ul>' +
      '<p>Real article body.</p>'
    const result = sanitizeArticleHtml(html)
    expect(result).not.toContain('<ul')
    expect(result).not.toContain('Copy link')
    expect(result).toContain('Real intro.')
    expect(result).toContain('Real article body.')
  })

  it('keeps a real content list with genuine text in every item', () => {
    const html = '<ul><li>First step</li><li>Second step</li></ul>'
    expect(sanitizeArticleHtml(html)).toBe(html)
  })
})
