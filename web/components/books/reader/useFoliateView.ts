'use client'

import { useEffect, useRef, useState, type RefObject } from 'react'
import {
  createFoliateView,
  type FoliateRelocateDetail,
  type FoliateView
} from '@/lib/books/foliate'
import {
  positionAt,
  resumeTarget,
  type ReaderPosition,
  type ReaderResume
} from '@/lib/books/readerPosition'
import { swipeDirection, tapDirection, type PageTurn } from '@/lib/books/readerSettings'

/** Where the reader is; `section` is the spine index, or the page index for a PDF. */
export interface ReaderLocation {
  fraction: number
  section: number
  cfi?: string
  tocLabel?: string
  tocHref?: string
  /** Set only on locations passed to `onRelocate`. */
  position?: ReaderPosition
}

type Status = 'loading' | 'ready' | 'error'

function toLocation(detail: FoliateRelocateDetail): ReaderLocation {
  return {
    fraction: detail.fraction ?? 0,
    section: detail.section?.current ?? 0,
    cfi: detail.cfi,
    tocLabel: detail.tocItem?.label,
    tocHref: detail.tocItem?.href
  }
}

// Opens at the stored position, else the stored percent, else the start.
async function openAt(view: FoliateView, resume: ReaderResume | undefined) {
  const target = resumeTarget(view.book?.sections, resume)
  try {
    if (target && 'fraction' in target) return await view.goToFraction(target.fraction)
    if (target) return await view.renderer!.goTo(target)
  } catch {
    // Fall through to the start.
  }
  await view.init({})
}

const TURNS: Record<PageTurn, (view: FoliateView) => unknown> = {
  left: (view) => view.goLeft(),
  right: (view) => view.goRight()
}

const KEY_TURNS: Partial<Record<string, PageTurn>> = { ArrowLeft: 'left', ArrowRight: 'right' }

function turn(view: FoliateView, direction: PageTurn | null | undefined) {
  if (direction) TURNS[direction](view)
}

function onArrowKey(view: FoliateView) {
  return (e: KeyboardEvent) => {
    // Leave keys alone while the contents or settings dialog is open.
    if (e.target instanceof Element && e.target.closest('[role="dialog"]')) return
    turn(view, KEY_TURNS[e.key])
  }
}

// Section documents live in iframes, so taps and keys never reach the page.
function attachSectionHandlers(view: FoliateView, doc: Document) {
  doc.addEventListener('click', (e) => {
    // The target belongs to the iframe's realm, so check against its Element.
    const ElementInFrame = doc.defaultView?.Element ?? Element
    if (e.defaultPrevented) return
    if (e.target instanceof ElementInFrame && e.target.closest('a[href]')) return
    if (doc.getSelection()?.isCollapsed === false) return
    // Map the iframe's coordinates to the window; fixed layouts scale the frame.
    const frame = doc.defaultView?.frameElement
    let x = e.clientX
    if (frame) {
      const rect = frame.getBoundingClientRect()
      const scale = frame.clientWidth ? rect.width / frame.clientWidth : 1
      x = rect.left + e.clientX * scale
    }
    const area = view.getBoundingClientRect()
    turn(view, tapDirection(x, area.left, area.width))
  })
  doc.addEventListener('keydown', onArrowKey(view))

  // The paginator swipes reflowable books itself; fixed layouts don't.
  if (!view.isFixedLayout) return
  let start: { x: number; y: number } | null = null
  doc.addEventListener('touchstart', (e) => {
    const touch = e.changedTouches[0]
    start = touch ? { x: touch.screenX, y: touch.screenY } : null
  })
  doc.addEventListener('touchend', (e) => {
    const touch = e.changedTouches[0]
    if (!start || !touch) return
    const direction = swipeDirection(touch.screenX - start.x, touch.screenY - start.y)
    start = null
    turn(view, direction)
  })
}

/**
 * Mounts a foliate-js view for `url` into `containerRef`, opening at
 * `initialPosition` (read once). Status isn't reset when `url` changes; key the
 * caller on it. `onRelocate` fires on page changes after the book has opened,
 * so opening alone never reports a position.
 */
export function useFoliateView(
  containerRef: RefObject<HTMLDivElement | null>,
  url: string,
  onRelocate?: (location: ReaderLocation) => void,
  initialPosition?: ReaderResume
) {
  const [view, setView] = useState<FoliateView | null>(null)
  const [status, setStatus] = useState<Status>('loading')
  const [location, setLocation] = useState<ReaderLocation | null>(null)
  const onRelocateRef = useRef(onRelocate)
  const initialPositionRef = useRef(initialPosition)
  useEffect(() => {
    onRelocateRef.current = onRelocate
  }, [onRelocate])

  useEffect(() => {
    let cancelled = false
    let created: FoliateView | null = null
    let opened = false
    let reason: string | undefined
    let onKey: (e: KeyboardEvent) => void = () => {}

    void (async () => {
      try {
        const v = await createFoliateView()
        if (cancelled) return
        created = v
        Object.assign(v.style, { display: 'block', width: '100%', height: '100%' })
        v.addEventListener('relocate', (e) => {
          const next = toLocation(e.detail)
          setLocation(next)
          // An 'anchor' relocate is a reflow (resize, image load, styles)
          // around the same spot, not reading.
          if (!opened || reason === 'anchor') return
          const position = positionAt(v.book?.sections, next.section, e.detail.range)
          onRelocateRef.current?.({ ...next, position })
        })
        v.addEventListener('load', (e) => {
          attachSectionHandlers(v, e.detail.doc)
        })
        v.addEventListener('click', (e) => {
          const area = v.getBoundingClientRect()
          turn(v, tapDirection(e.clientX, area.left, area.width))
        })
        containerRef.current!.append(v)
        await v.open(url)
        if (cancelled) return
        // The view re-emits the renderer's relocate without its reason; a
        // capture listener runs first at the target, so it sees it in time.
        v.renderer?.addEventListener?.('relocate', (e) => (reason = e.detail.reason), {
          capture: true
        })
        setView(v)
        setStatus('ready')
        onKey = onArrowKey(v)
        window.addEventListener('keydown', onKey)
        await openAt(v, initialPositionRef.current)
        opened = true
      } catch {
        if (!cancelled) setStatus('error')
      }
    })()

    return () => {
      cancelled = true
      window.removeEventListener('keydown', onKey)
      created?.close()
      created?.book?.destroy?.()
      created?.remove()
    }
  }, [containerRef, url])

  return { view, status, location }
}
