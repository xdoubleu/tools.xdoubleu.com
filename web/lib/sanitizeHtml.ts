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

function stripLeftoverShareWidgets(doc: Document): void {
  doc.querySelectorAll('ul, ol').forEach((list) => {
    const items = list.querySelectorAll(':scope > li')
    if (items.length > 0 && Array.from(items).every(isJunkListItem)) {
      list.remove()
    }
  })
}

// Some sources (e.g. the Claude Blog) open an article with a decorative hero
// image that carries no information the title doesn't already give (#911).
// Only the leading image goes — an image with any text before it is part of
// the article body, and an item whose entire content is one image (a comic
// feed) keeps it.
function stripHeroImage(doc: Document): void {
  const img = doc.querySelector('img')
  if (img === null || doc.body.textContent!.trim() === '') return

  const before = doc.createRange()
  before.setStart(doc.body, 0)
  before.setEndBefore(img)
  if (before.toString().trim() !== '') return

  // Drop the wrappers the image left empty behind it, not just the <img>.
  let block: Element = img
  while (
    block.parentElement !== null &&
    block.parentElement !== doc.body &&
    block.parentElement.childElementCount === 1 &&
    block.parentElement.textContent!.trim() === ''
  ) {
    block = block.parentElement
  }
  block.remove()
}

// Articles are sometimes extracted with a trailing newsletter-signup section
// still attached (#911 — the Claude Blog's "Transform how your organization
// operates with Claude" block). A <form> is the reliable marker: article
// prose has none. From the form, climb to the whole section by walking up
// while the parent adds little text of its own — every newsletter wrapper
// does, while the ancestor holding the actual article multiplies it.
//
// ponytail: text-growth heuristic, no per-site rules; if a source starts
// losing real content to it, match the section by marker text instead.
function stripNewsletterSignups(doc: Document): void {
  const totalTextLen = doc.body.textContent!.trim().length
  doc.querySelectorAll('form').forEach((form) => {
    let block: Element = form
    for (;;) {
      // Non-null until the walk reaches <body>, which ends it.
      const parent = block.parentElement!
      if (parent === doc.body) break
      const parentLen = parent.textContent!.trim().length
      // Never swallow a parent that holds the bulk of the article.
      if (parentLen > 2 * block.textContent!.trim().length || parentLen > totalTextLen / 2) break
      block = parent
    }
    block.remove()
  })
}

export function sanitizeArticleHtml(html: string): string {
  const sanitized = DOMPurify.sanitize(html, {
    FORBID_TAGS: ['style', 'nav', 'header'],
    KEEP_CONTENT: false
  })
  const doc = new DOMParser().parseFromString(sanitized, 'text/html')
  stripLeftoverShareWidgets(doc)
  stripNewsletterSignups(doc)
  stripHeroImage(doc)
  return doc.body.innerHTML.replace(/\sloading="lazy"/g, '')
}
