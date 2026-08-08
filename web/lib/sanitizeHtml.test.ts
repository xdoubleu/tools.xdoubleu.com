import { sanitizeArticleHtml } from './sanitizeHtml'

describe('sanitizeArticleHtml', () => {
  it('strips style tags', () => {
    expect(sanitizeArticleHtml('<style>body{width:1px}</style><p>hi</p>')).toBe('<p>hi</p>')
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
