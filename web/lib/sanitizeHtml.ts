import DOMPurify from 'dompurify'

// Third-party HTML (email/RSS) can carry a <style> block; DOMPurify allows
// it by default, but <style> isn't scoped to its container and leaks CSS
// (e.g. `body { width: ... }`) onto the whole app (#715).
//
// Some scraped sources (e.g. the Claude Blog) ship <img loading="lazy"> with
// no width/height, so the browser reserves zero space and each image pops
// in mid-scroll, shoving the article down — read as scroll/twitch (#862).
// The images are already fully in the DOM when the dialog opens, so eager
// loading settles layout once up front instead of drip-feeding shifts.
//
// Readability-based extraction can misjudge a source page's markup and
// include the site's own <nav>/<header> menu as if it were article content
// (#892). DOMPurify allows both by default and keeps their text content when
// forbidding just the tag, so KEEP_CONTENT is also disabled to drop the menu
// text itself rather than leaving it as unwrapped text.
//
// Some sites (e.g. Webflow exports) don't mark their share/nav widgets with
// <nav>/<header> at all — just a plain <ul> of empty icon <li>s plus one
// <li> with "Share" / "Copy link" / the raw article URL. FORBID_TAGS can't
// catch that, so stripLeftoverShareWidgets drops any <ul>/<ol> whose every
// <li> is empty or matches that boilerplate text (#906).
const SHARE_WIDGET_TEXT = new Set(['share', 'copy link', 'tweet', 'follow'])
const BARE_URL_RE = /^https?:\/\/\S+$/i

function isJunkListItem(li: Element): boolean {
  const paragraphs = li.querySelectorAll('p')
  const chunks =
    paragraphs.length > 0 ? Array.from(paragraphs, (p) => p.textContent!) : [li.textContent!]
  return chunks.every((chunk) => {
    const text = chunk.trim().toLowerCase()
    return text === '' || SHARE_WIDGET_TEXT.has(text) || BARE_URL_RE.test(text)
  })
}

function stripLeftoverShareWidgets(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc.querySelectorAll('ul, ol').forEach((list) => {
    const items = list.querySelectorAll(':scope > li')
    if (items.length > 0 && Array.from(items).every(isJunkListItem)) {
      list.remove()
    }
  })
  return doc.body.innerHTML
}

export function sanitizeArticleHtml(html: string): string {
  const sanitized = DOMPurify.sanitize(html, {
    FORBID_TAGS: ['style', 'nav', 'header'],
    KEEP_CONTENT: false
  })
  return stripLeftoverShareWidgets(sanitized).replace(/\sloading="lazy"/g, '')
}
