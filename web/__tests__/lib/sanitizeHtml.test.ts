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

  it('drops the leading hero image and its empty wrapper, keeps body images (#911)', () => {
    const html =
      '<div><p><img src="hero.svg" alt=""/></p><p><em>Subtitle text here.</em></p></div>' +
      '<p>Article body.</p><figure><img src="inline.png" alt="chart"/></figure>'
    const result = sanitizeArticleHtml(html)
    expect(result).not.toContain('hero.svg')
    expect(result).not.toContain('<p></p>')
    expect(result).toContain('inline.png')
    expect(result).toContain('Subtitle text here.')
  })

  it('keeps an image that has article text before it (#911)', () => {
    const html = '<p>Intro paragraph.</p><p><img src="body.png" alt=""></p>'
    expect(sanitizeArticleHtml(html)).toBe(html)
  })

  it('keeps an item whose entire content is one image (#911)', () => {
    expect(sanitizeArticleHtml('<img src="comic.png" alt=""/>')).toContain('comic.png')
  })

  it('drops a trailing newsletter signup section around its form (#911)', () => {
    const html =
      '<div id="main"><div id="body"><p>Real article body that carries most of the text of this ' +
      'page, several sentences long so the section heuristic has something to weigh against.</p>' +
      '<p>More real article body, still going, still the bulk of the content on this page.</p>' +
      '</div><div class="section"><h2>Transform how your organization operates with Claude</h2>' +
      '<div><p>Get the developer newsletter</p><p>Product updates, delivered monthly.</p>' +
      '<div><form action="/subscribe"><p>Please provide your email address.</p></form>' +
      '<div><p>Thank you! You’re subscribed.</p></div></div></div></div></div>'
    const result = sanitizeArticleHtml(html)
    expect(result).not.toContain('Transform how your organization')
    expect(result).not.toContain('<form')
    expect(result).not.toContain('Get the developer newsletter')
    expect(result).toContain('Real article body')
    expect(result).toContain('More real article body')
  })

  it('never lets the newsletter strip swallow the article around the form', () => {
    const html = '<div><p>Short body.</p><form action="/x"><p>Email</p></form></div>'
    const result = sanitizeArticleHtml(html)
    expect(result).not.toContain('<form')
    expect(result).toContain('Short body.')
  })

  it('drops a bare top-level form without climbing out of the article', () => {
    const html = '<p>Real body.</p><form action="/x"><p>Sign up for the newsletter.</p></form>'
    const result = sanitizeArticleHtml(html)
    expect(result).toBe('<p>Real body.</p>')
  })

  it('stops climbing at a wrapper holding more than half the article text', () => {
    const html =
      '<div><div><form action="/x"><p>Subscribe to the newsletter for updates.</p></form>' +
      '<p>Article text sitting right beside the form.</p></div><p>Tail.</p></div>'
    const result = sanitizeArticleHtml(html)
    expect(result).not.toContain('<form')
    expect(result).toContain('Article text sitting right beside the form.')
    expect(result).toContain('Tail.')
  })
})
