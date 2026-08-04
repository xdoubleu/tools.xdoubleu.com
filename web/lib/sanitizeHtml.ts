import DOMPurify from 'dompurify'

// Third-party HTML (email/RSS) can carry a <style> block; DOMPurify allows
// it by default, but <style> isn't scoped to its container and leaks CSS
// (e.g. `body { width: ... }`) onto the whole app (#715).
export function sanitizeArticleHtml(html: string): string {
  return DOMPurify.sanitize(html, { FORBID_TAGS: ['style'] })
}
