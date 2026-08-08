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
export function sanitizeArticleHtml(html: string): string {
  return DOMPurify.sanitize(html, {
    FORBID_TAGS: ['style', 'nav', 'header'],
    KEEP_CONTENT: false
  }).replace(/\sloading="lazy"/g, '')
}
