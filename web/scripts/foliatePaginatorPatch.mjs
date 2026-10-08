// Defensive guards for foliate-js's paginator.js, applied at copy time
// (copy-foliate.mjs). foliate-js is pinned to upstream HEAD, which has no fix:
// on mobile, a swipe or scroll/resize can land while a section's iframe document
// is blank or torn down, and the paginator reads a null document or unseeded
// scroll position and crashes, aborting render to a blank 0% page. These match
// the guards the readest project carries on its fork; kept as a build-time patch.

/**
 * Applies the null-document / unseeded-scroll guards to paginator.js source.
 * Throws if any expected snippet is missing, so a foliate-js bump that changes
 * the surrounding code fails the build loudly instead of silently dropping a guard.
 */
export function applyPaginatorPatch(source) {
  let src = source

  src = replaceOnce(
    src,
    'const getVisibleRange = (doc, start, end, mapRect) => {\n    // first get all visible nodes',
    'const getVisibleRange = (doc, start, end, mapRect) => {\n    // A resize/scroll can run after the view document detached or while a\n    // section is still loading; there is nothing to measure without a body.\n    if (!doc?.body) return\n    // first get all visible nodes',
    'getVisibleRange null-doc guard'
  )

  src = replaceOnce(
    src,
    'const getDirection = doc => {\n    const { defaultView } = doc',
    'const getDirection = doc => {\n    // The iframe document can be blank while a section loads or is torn down;\n    // getComputedStyle on a missing body throws. Fall back to horizontal-ltr.\n    if (!doc?.defaultView || !doc.body) return { vertical: false, rtl: false }\n    const { defaultView } = doc',
    'getDirection blank-doc guard'
  )

  src = replaceOnce(
    src,
    'const getBackground = doc => {\n    const bodyStyle = doc.defaultView.getComputedStyle(doc.body)',
    'const getBackground = doc => {\n    // A blank/detached document has no computed style to read.\n    if (!doc?.defaultView || !doc.body) return \'\'\n    const bodyStyle = doc.defaultView.getComputedStyle(doc.body)',
    'getBackground blank-doc guard'
  )

  src = replaceOnce(
    src,
    'const setStylesImportant = (el, styles) => {\n    const { style } = el',
    'const setStylesImportant = (el, styles) => {\n    // el is the documentElement, null while a view document is blank.\n    if (!el) return\n    const { style } = el',
    'setStylesImportant blank-el guard'
  )

  src = replaceOnce(
    src,
    "this.#iframe.addEventListener('load', () => {\n                const doc = this.document\n                afterLoad?.(doc)",
    `this.#iframe.addEventListener('load', () => {
                const doc = this.document
                // The section document can still be blank on a racing or failed
                // load; render nothing rather than styling a null body.
                if (!doc?.documentElement || !doc.body) return resolve()
                afterLoad?.(doc)`,
    'View.load blank-doc guard'
  )

  src = replaceOnce(
    src,
    'scrollBy(dx, dy) {\n        const delta = this.#vertical ? dy : dx',
    'scrollBy(dx, dy) {\n        // Bounds are seeded on the first settled scroll; an early swipe before\n        // that destructures undefined. Drop it; settled scrolls reseed.\n        if (!this.#scrollBounds) return\n        const delta = this.#vertical ? dy : dx',
    'scrollBy unseeded-bounds guard'
  )

  src = replaceOnce(
    src,
    'snap(vx, vy) {\n        const velocity = this.#vertical ? vy : vx',
    'snap(vx, vy) {\n        // Same unseeded-bounds guard as scrollBy (see above).\n        if (!this.#scrollBounds) return\n        const velocity = this.#vertical ? vy : vx',
    'snap unseeded-bounds guard'
  )

  return src
}

/** Replaces `needle` (must occur exactly once); throws if missing or duplicated. */
function replaceOnce(source, needle, replacement, label) {
  const count = source.split(needle).length - 1
  if (count !== 1) {
    throw new Error(
      `foliate-js paginator ${label}: expected 1 occurrence of the patch anchor, found ${count}. ` +
        'The foliate-js version may have changed; re-check the guard.'
    )
  }
  return source.replace(needle, replacement)
}
