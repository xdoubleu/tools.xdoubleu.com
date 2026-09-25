import DOMPurify from 'dompurify'

// Sanitizing third-party (email/RSS/scraped) HTML:
// - <style> is forbidden: it isn't scoped and leaks CSS onto the app.
// - Images load eagerly so layout settles once instead of shifting mid-read.
// - <nav>/<header> are dropped with their text (KEEP_CONTENT off), since
//   extraction sometimes includes the site menu.
// - stripLeftoverShareWidgets drops lists that are only share/nav boilerplate.
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

// Drops a leading decorative hero image, unless the item is only that image.
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

// Drops a trailing newsletter-signup section: find its <form>, then climb
// while the parent adds little text of its own.
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
