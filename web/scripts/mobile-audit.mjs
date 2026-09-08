#!/usr/bin/env node
/**
 * Measures the mobile failure modes that only a real viewport can settle —
 * horizontal overflow and tap-target geometry — for the routes given on the
 * command line. The `mobile-review` skill runs this; the static checks in that
 * skill cover what a browser can't see (hover-only affordances, theme tokens).
 *
 * Usage: npm run mobile:audit -- /trains /books [--base-url ...] [--json]
 */
import { chromium, devices } from '@playwright/test'

const MIN_TAP_PX = 44 // Apple HIG / WCAG 2.2 AAA target size.
const MIN_TAP_GAP_PX = 8
const MIN_INPUT_FONT_PX = 16 // Below this, iOS Safari zooms on focus.
const OVERFLOW_TOLERANCE_PX = 1 // Sub-pixel layout rounding is not a finding.

function parseArgs(argv) {
  const paths = []
  const opts = {
    baseUrl: process.env.MOBILE_AUDIT_BASE_URL ?? 'http://localhost:3000',
    storageState: undefined,
    width: 375,
    height: 667,
    themes: ['light', 'dark'],
    json: false
  }
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i]
    if (arg === '--json') opts.json = true
    else if (arg === '--base-url') opts.baseUrl = argv[++i]
    else if (arg === '--storage-state') opts.storageState = argv[++i]
    else if (arg === '--width') opts.width = Number(argv[++i])
    else if (arg === '--height') opts.height = Number(argv[++i])
    else if (arg === '--theme') opts.themes = [argv[++i]]
    else if (arg.startsWith('-')) throw new Error(`unknown flag: ${arg}`)
    else paths.push(arg)
  }
  if (paths.length === 0) {
    throw new Error(
      'no routes given — e.g. npm run mobile:audit -- /trains /books'
    )
  }
  return { paths, opts }
}

/** Runs in the page. Returns findings for one route at one theme. */
/* eslint-disable no-undef -- the body below is serialized and runs in the page, not in Node. */
function collect({ minTap, minGap, minInputFont, tolerance }) {
  const label = (el) => {
    const id = el.id ? `#${el.id}` : ''
    const cls =
      typeof el.className === 'string' && el.className
        ? `.${el.className.trim().split(/\s+/).slice(0, 3).join('.')}`
        : ''
    const text = (el.textContent ?? '').trim().replace(/\s+/g, ' ').slice(0, 40)
    return `${el.tagName.toLowerCase()}${id}${cls}${text ? ` — "${text}"` : ''}`
  }
  const visible = (el) => {
    const r = el.getBoundingClientRect()
    if (r.width === 0 || r.height === 0) return false
    const s = getComputedStyle(el)
    return (
      s.visibility !== 'hidden' && s.display !== 'none' && s.opacity !== '0'
    )
  }

  const viewportWidth = document.documentElement.clientWidth
  const findings = []

  // Without this tag the page lays out at ~980px and is scaled down, which
  // makes every other measurement below meaningless as well as being the
  // single worst mobile defect on its own.
  const viewportMeta = document.querySelector('meta[name="viewport"]')
  if (!viewportMeta) {
    findings.push({
      severity: 'broken',
      kind: 'no-viewport-meta',
      detail: `no <meta name="viewport"> — the page lays out at ${viewportWidth}px and is scaled down to fit, so all text and targets render tiny`,
      elements: []
    })
  } else if (/user-scalable\s*=\s*no|maximum-scale\s*=\s*1/.test(viewportMeta.content)) {
    findings.push({
      severity: 'degraded',
      kind: 'zoom-disabled',
      detail: `viewport meta blocks pinch-zoom ("${viewportMeta.content}"), which fails WCAG 1.4.4`,
      elements: []
    })
  }

  // Horizontal overflow: report the outermost offenders only, so one wide
  // child doesn't produce a finding for every ancestor that contains it.
  const scrollWidth = document.documentElement.scrollWidth
  if (scrollWidth > viewportWidth + tolerance) {
    const wide = [...document.querySelectorAll('body *')].filter((el) => {
      if (!visible(el)) return false
      const r = el.getBoundingClientRect()
      return r.right > viewportWidth + tolerance || r.left < -tolerance
    })
    const outermost = wide.filter((el) => !wide.some((o) => o !== el && o.contains(el)))
    findings.push({
      severity: 'broken',
      kind: 'horizontal-overflow',
      detail: `page scrolls sideways: content is ${Math.round(scrollWidth)}px wide in a ${viewportWidth}px viewport`,
      elements: outermost.slice(0, 8).map((el) => {
        const r = el.getBoundingClientRect()
        return `${label(el)} (${Math.round(r.width)}px wide, right edge at ${Math.round(r.right)}px)`
      })
    })
  }

  const targets = [
    ...document.querySelectorAll(
      'a[href], button, input, select, textarea, summary, [role="button"], [role="link"], [role="tab"], [role="checkbox"], [role="switch"], [tabindex]:not([tabindex="-1"])'
    )
  ].filter(
    (el) => visible(el) && el.type !== 'hidden' && !el.disabled
  )

  // A checkbox/radio reached through a <label> — wrapping it, or associated
  // by `for` — has that label as its real tap target, same as a link inside
  // a larger tappable card.
  const labelFor = (el) => {
    if (el.tagName !== 'INPUT' || (el.type !== 'checkbox' && el.type !== 'radio')) return null
    return (el.id && document.querySelector(`label[for="${CSS.escape(el.id)}"]`)) || el.closest('label')
  }

  const small = targets
    .map((el) => ({ el, r: el.getBoundingClientRect() }))
    // A link inside a larger tappable card is fine — the card is the target.
    .filter(({ el, r }) => {
      if (r.width >= minTap && r.height >= minTap) return false
      const bigEnoughAncestor = targets.some((o) => {
        if (o === el || !o.contains(el)) return false
        const or = o.getBoundingClientRect()
        return or.width >= minTap && or.height >= minTap
      })
      if (bigEnoughAncestor) return false
      const label = labelFor(el)
      if (label) {
        const lr = label.getBoundingClientRect()
        if (lr.width >= minTap && lr.height >= minTap) return false
      }
      return true
    })
  if (small.length > 0) {
    findings.push({
      severity: 'degraded',
      kind: 'small-tap-target',
      detail: `${small.length} tappable element(s) smaller than ${minTap}×${minTap}px`,
      elements: small
        .slice(0, 12)
        .map(({ el, r }) => `${label(el)} (${Math.round(r.width)}×${Math.round(r.height)}px)`)
    })
  }

  const crowded = []
  const boxes = targets.map((el) => ({ el, r: el.getBoundingClientRect() }))
  for (let i = 0; i < boxes.length; i++) {
    for (let j = i + 1; j < boxes.length; j++) {
      const a = boxes[i]
      const b = boxes[j]
      if (a.el.contains(b.el) || b.el.contains(a.el)) continue
      const dx = Math.max(0, Math.max(a.r.left - b.r.right, b.r.left - a.r.right))
      const dy = Math.max(0, Math.max(a.r.top - b.r.bottom, b.r.top - a.r.bottom))
      if (dx === 0 && dy === 0) continue // overlapping/nested layouts, not a gap
      const gap = dx === 0 ? dy : dy === 0 ? dx : Math.hypot(dx, dy)
      if (gap < minGap) crowded.push(`${label(a.el)} ↔ ${label(b.el)} (${Math.round(gap)}px apart)`)
    }
  }
  if (crowded.length > 0) {
    findings.push({
      severity: 'degraded',
      kind: 'crowded-tap-targets',
      detail: `${crowded.length} pair(s) of tap targets closer than ${minGap}px`,
      elements: crowded.slice(0, 8)
    })
  }

  const zoomers = [...document.querySelectorAll('input, select, textarea')]
    .filter((el) => visible(el) && parseFloat(getComputedStyle(el).fontSize) < minInputFont)
    .map((el) => `${label(el)} (${getComputedStyle(el).fontSize})`)
  if (zoomers.length > 0) {
    findings.push({
      severity: 'degraded',
      kind: 'ios-focus-zoom',
      detail: `${zoomers.length} field(s) render below ${minInputFont}px, so iOS Safari zooms in on focus and does not zoom back`,
      elements: zoomers.slice(0, 8)
    })
  }

  return findings
}
/* eslint-enable no-undef */

const { paths, opts } = parseArgs(process.argv.slice(2))
// Chromium comes from `npx playwright install chromium`; an environment that
// ships its own build points at it with CHROMIUM_EXECUTABLE_PATH instead.
let browser
try {
  browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_EXECUTABLE_PATH || undefined
  })
} catch (err) {
  console.error(
    `could not launch Chromium: ${err.message}\n` +
      'Run `npx playwright install chromium` from web/, or set ' +
      'CHROMIUM_EXECUTABLE_PATH to an existing Chromium binary.'
  )
  process.exit(1)
}
const context = await browser.newContext({
  ...devices['iPhone SE'],
  viewport: { width: opts.width, height: opts.height },
  storageState: opts.storageState
})

const results = []
for (const theme of opts.themes) {
  // eslint-disable-next-line no-undef -- runs in the page; mirrors THEME_KEY in lib/theme.ts.
  await context.addInitScript((t) => localStorage.setItem('theme', t), theme)
  for (const path of paths) {
    const url = new URL(path, opts.baseUrl).toString()
    const page = await context.newPage()
    try {
      const response = await page.goto(url, { waitUntil: 'networkidle' })
      const status = response?.status() ?? 0
      if (status >= 400) {
        results.push({ path, theme, error: `HTTP ${status}` })
        continue
      }
      const findings = await page.evaluate(collect, {
        minTap: MIN_TAP_PX,
        minGap: MIN_TAP_GAP_PX,
        minInputFont: MIN_INPUT_FONT_PX,
        tolerance: OVERFLOW_TOLERANCE_PX
      })
      results.push({ path, theme, findings })
    } catch (err) {
      results.push({ path, theme, error: err.message })
    } finally {
      await page.close()
    }
  }
}
await browser.close()

if (opts.json) {
  console.log(JSON.stringify(results, null, 2))
} else {
  for (const { path, theme, findings, error } of results) {
    const head = `${path}  [${theme}, ${opts.width}×${opts.height}]`
    if (error) {
      console.log(`\n${head}\n  could not audit: ${error}`)
      continue
    }
    if (findings.length === 0) {
      console.log(`\n${head}\n  no measurable mobile issues`)
      continue
    }
    console.log(`\n${head}`)
    for (const f of findings) {
      console.log(`  [${f.severity}] ${f.kind}: ${f.detail}`)
      for (const el of f.elements) console.log(`      ${el}`)
    }
  }
  console.log('')
}

const broken = results.some((r) => r.findings?.some((f) => f.severity === 'broken'))
const failed = results.some((r) => r.error)
process.exit(broken || failed ? 1 : 0)
