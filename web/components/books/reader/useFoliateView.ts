'use client'

import { useEffect, useRef, useState, type RefObject } from 'react'
import {
  createFoliateView,
  type FoliateRelocateDetail,
  type FoliateTocItem,
  type FoliateView
} from '@/lib/books/foliate'
import { swipeDirection, tapDirection, type PageTurn } from '@/lib/books/readerSettings'

/** Where the reader is; `section` is the spine index, or the page index for a PDF. */
export interface ReaderLocation {
  fraction: number
  section: number
  cfi?: string
  tocLabel?: string
  tocHref?: string
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

function turn(view: FoliateView, direction: PageTurn) {
  if (direction === 'left') view.goLeft()
  else view.goRight()
}

function onArrowKey(view: FoliateView) {
  return (e: KeyboardEvent) => {
    // Leave keys alone while the contents or settings dialog is open.
    if (e.target instanceof Element && e.target.closest('[role="dialog"]')) return
    if (e.key === 'ArrowLeft') turn(view, 'left')
    else if (e.key === 'ArrowRight') turn(view, 'right')
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
    const direction = tapDirection(x, area.left, area.width)
    if (direction) turn(view, direction)
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
    if (direction) turn(view, direction)
  })
}

/**
 * Mounts a foliate-js view for `url` into `containerRef`, opening at the start.
 * `onRelocate` fires on every page change.
 */
export function useFoliateView(
  containerRef: RefObject<HTMLDivElement | null>,
  url: string,
  onRelocate?: (location: ReaderLocation) => void
) {
  const [view, setView] = useState<FoliateView | null>(null)
  const [status, setStatus] = useState<Status>('loading')
  const [toc, setToc] = useState<FoliateTocItem[]>([])
  const [location, setLocation] = useState<ReaderLocation | null>(null)
  const onRelocateRef = useRef(onRelocate)
  useEffect(() => {
    onRelocateRef.current = onRelocate
  }, [onRelocate])

  useEffect(() => {
    let cancelled = false
    let created: FoliateView | null = null
    let onKey: ((e: KeyboardEvent) => void) | null = null
    setStatus('loading')

    void (async () => {
      try {
        const v = await createFoliateView()
        if (cancelled) return
        created = v
        Object.assign(v.style, { display: 'block', width: '100%', height: '100%' })
        v.addEventListener('relocate', (e) => {
          const next = toLocation(e.detail)
          setLocation(next)
          onRelocateRef.current?.(next)
        })
        v.addEventListener('load', (e) => {
          attachSectionHandlers(v, e.detail.doc)
        })
        v.addEventListener('click', (e) => {
          const area = v.getBoundingClientRect()
          const direction = tapDirection(e.clientX, area.left, area.width)
          if (direction) turn(v, direction)
        })
        containerRef.current?.append(v)
        await v.open(url)
        if (cancelled) return
        setToc(v.book?.toc ?? [])
        setView(v)
        setStatus('ready')
        onKey = onArrowKey(v)
        window.addEventListener('keydown', onKey)
        await v.init({})
      } catch {
        if (!cancelled) setStatus('error')
      }
    })()

    return () => {
      cancelled = true
      if (onKey) window.removeEventListener('keydown', onKey)
      created?.close()
      created?.book?.destroy?.()
      created?.remove()
    }
  }, [containerRef, url])

  return { view, status, toc, location }
}
