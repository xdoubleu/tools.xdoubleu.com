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
export function sanitizeArticleHtml(html: string): string {
  return DOMPurify.sanitize(html, { FORBID_TAGS: ['style'] }).replace(/\sloading="lazy"/g, '')
}
