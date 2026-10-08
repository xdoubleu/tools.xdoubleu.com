import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { applyPaginatorPatch } from '../../../scripts/foliatePaginatorPatch.mjs'

// The patch anchors must match the vendored foliate-js we copy at build time,
// so the guards are validated against the real pinned source, not a fixture.
// jest intercepts `require.resolve`, so locate it under web/node_modules by path.
const vendoredPath = join(process.cwd(), 'node_modules', 'foliate-js', 'paginator.js')
const vendoredSource = readFileSync(vendoredPath, 'utf8')

describe('applyPaginatorPatch on the vendored foliate-js', () => {
  const patched = applyPaginatorPatch(vendoredSource)

  it('produces valid JavaScript', () => {
    // paginator.js is an ES module (export class); parse the body as a plain
    // script after dropping the module keyword to confirm the patch keeps it valid.
    expect(() => new Function(patched.replace('export class', 'class'))).not.toThrow()
  })

  it('guards getVisibleRange against a null document', () => {
    expect(patched).toMatch(
      /getVisibleRange = \(doc, start, end, mapRect\) => \{\n(?: {0,4}\/\/[^\n]*\n)+ {0,4}if \(!doc\?\.body\) return/
    )
    // the crash site must sit behind the guard
    expect(patched.indexOf('createTreeWalker')).toBeGreaterThan(
      patched.indexOf('if (!doc?.body) return')
    )
  })

  it('guards getDirection and getBackground against a blank document', () => {
    expect(patched).toMatch(
      /getDirection = doc => \{\n(?: {0,4}\/\/[^\n]*\n)+ {0,4}if \(!doc\?\.defaultView \|\| !doc\.body\) return \{ vertical: false, rtl: false \}/
    )
    expect(patched).toMatch(
      /getBackground = doc => \{\n(?: {0,4}\/\/[^\n]*\n)+ {0,4}if \(!doc\?\.defaultView \|\| !doc\.body\) return ''/
    )
  })

  it('guards setStylesImportant against a null element', () => {
    expect(patched).toMatch(
      /setStylesImportant = \(el, styles\) => \{\n(?: {0,4}\/\/[^\n]*\n)+ {0,4}if \(!el\) return/
    )
  })

  it('returns before styling a still-loading section document', () => {
    expect(patched).toMatch(
      /if \(!doc\?\.documentElement \|\| !doc\.body\) return resolve\(\)\n\s+afterLoad\?\.\(doc\)/
    )
  })

  it('drops an early swipe that precedes the first settled scroll', () => {
    expect(patched).toMatch(
      /scrollBy\(dx, dy\) \{\n(?: {0,8}\/\/[^\n]*\n)+ {0,8}if \(!this\.#scrollBounds\) return/
    )
    expect(patched).toMatch(
      /snap\(vx, vy\) \{\n(?: {0,8}\/\/[^\n]*\n)+ {0,8}if \(!this\.#scrollBounds\) return/
    )
  })

  it('panics when a bump rewrites an anchor without the guard', () => {
    // A foliate-js bump that rewrites an anchor must fail the build loudly.
    const drifted = vendoredSource.replace(
      'const setStylesImportant = (el, styles) => {',
      'const setStylesImportant = (el, styles, extra) => {'
    )
    expect(() => applyPaginatorPatch(drifted)).toThrow(/setStylesImportant blank-el guard/)
  })
})
