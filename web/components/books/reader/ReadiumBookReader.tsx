'use client'

import { useEffect, useRef, useState } from 'react'
import { EpubNavigator, EpubPreferences } from '@readium/navigator'
import { Link, type Locator, type Publication } from '@readium/shared'
import { useLocalStorage } from '@/hooks/useLocalStorage'
import type { FoliateTocItem } from '@/lib/books/foliate'
import {
  loadPublication,
  locatorToPosition,
  overallFraction,
  readiumPreferences,
  resumeLocator,
  sectionIndex,
  tocItems
} from '@/lib/books/readium'
import type { ReaderResume } from '@/lib/books/readerPosition'
import {
  READER_FONT_SIZE_DEFAULT,
  tapDirection,
  type PageTurn,
  type ReaderTheme
} from '@/lib/books/readerSettings'
import ReaderControls from './ReaderControls'
import ReaderShell, { type ReaderStatus } from './ReaderShell'
import type { ReaderLocation } from './useFoliateView'

interface ReadiumBookReaderProps {
  bookId: string
  title: string
  onClose: () => void
  /** Fires on every page change after the book opens. */
  onRelocate?: (location: ReaderLocation) => void
  /** Where to open; read once. Defaults to the start. */
  initialPosition?: ReaderResume
}

function appTheme(): ReaderTheme {
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'
}

const noop = () => {}

function tocLabelAt(items: FoliateTocItem[], href: string): FoliateTocItem | undefined {
  let found: FoliateTocItem | undefined
  for (const item of items) {
    if (item.href?.split('#')[0] === href) found = item
    found = tocLabelAt(item.subitems ?? [], href) ?? found
  }
  return found
}

/** Full-screen Readium reader for an original EPUB served as a WebPub. */
export default function ReadiumBookReader({
  bookId,
  title,
  onClose,
  onRelocate,
  initialPosition
}: ReadiumBookReaderProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [navigator, setNavigator] = useState<EpubNavigator | null>(null)
  const [toc, setToc] = useState<FoliateTocItem[]>([])
  const [status, setStatus] = useState<ReaderStatus>('loading')
  const [location, setLocation] = useState<ReaderLocation | null>(null)
  const [storedTheme, setTheme] = useLocalStorage<ReaderTheme | null>('books:reader-theme', null)
  const [fontSize, setFontSize] = useLocalStorage(
    'books:reader-font-size',
    READER_FONT_SIZE_DEFAULT
  )
  const [defaultTheme] = useState(appTheme)
  const theme = storedTheme ?? defaultTheme
  const prefsRef = useRef(readiumPreferences(theme, fontSize))
  const onRelocateRef = useRef(onRelocate)
  const initialPositionRef = useRef(initialPosition)
  useEffect(() => {
    onRelocateRef.current = onRelocate
  }, [onRelocate])

  useEffect(() => {
    let cancelled = false
    let created: EpubNavigator | null = null
    let opened = false
    let onKey: (e: KeyboardEvent) => void = noop

    const handleLocator = async (pub: Publication, locator: Locator, items: FoliateTocItem[]) => {
      const entry = tocLabelAt(items, locator.href.split('#')[0]!)
      const next: ReaderLocation = {
        fraction: overallFraction(pub, locator),
        section: Math.max(0, sectionIndex(pub, locator.href)),
        tocLabel: entry?.label,
        tocHref: entry?.href
      }
      if (!cancelled) setLocation(next)
      if (!opened) return
      const position = await locatorToPosition(pub, locator)
      if (!cancelled) onRelocateRef.current?.({ ...next, position })
    }

    void (async () => {
      try {
        const pub = await loadPublication(bookId)
        const items = tocItems(pub.manifest.toc?.items)
        const [initial, positions] = await Promise.all([
          resumeLocator(pub, initialPositionRef.current),
          pub.positionsFromManifest().catch(() => [])
        ])
        if (cancelled) return
        const turn = (direction: PageTurn | null) => {
          if (direction === 'left') created?.goBackward(false, noop)
          if (direction === 'right') created?.goForward(false, noop)
        }
        const container = containerRef.current!
        const nav = new EpubNavigator(
          container,
          pub,
          {
            frameLoaded: noop,
            positionChanged: (locator) => void handleLocator(pub, locator, items),
            timelineItemChanged: noop,
            tap: (e) => {
              const area = container.getBoundingClientRect()
              const direction = tapDirection(e.x, area.left, area.width)
              turn(direction)
              return direction !== null
            },
            click: () => false,
            zoom: noop,
            miscPointer: noop,
            scroll: noop,
            customEvent: noop,
            handleLocator: () => false,
            textSelected: noop,
            contentProtection: noop,
            contextMenu: noop,
            peripheral: noop
          },
          positions,
          initial,
          { preferences: prefsRef.current, defaults: {} }
        )
        created = nav
        await nav.load()
        if (cancelled) return
        onKey = (e) => {
          if (e.target instanceof Element && e.target.closest('[role="dialog"]')) return
          if (e.key === 'ArrowLeft') turn('left')
          if (e.key === 'ArrowRight') turn('right')
        }
        window.addEventListener('keydown', onKey)
        setToc(items)
        setNavigator(nav)
        setStatus('ready')
        opened = true
      } catch {
        if (!cancelled) setStatus('error')
      }
    })()

    return () => {
      cancelled = true
      window.removeEventListener('keydown', onKey)
      void created?.destroy()
    }
  }, [bookId])

  useEffect(() => {
    prefsRef.current = readiumPreferences(theme, fontSize)
    void navigator?.submitPreferences(new EpubPreferences(prefsRef.current))
  }, [navigator, theme, fontSize])

  return (
    <ReaderShell
      title={title}
      onClose={onClose}
      containerRef={containerRef}
      status={status}
      theme={theme}
      fraction={location?.fraction ?? 0}
      tocLabel={location?.tocLabel}
      controls={
        navigator && (
          <ReaderControls
            toc={toc}
            onGoTo={(href) => navigator.goLink(new Link({ href }), false, noop)}
            reflowable
            currentHref={location?.tocHref}
            theme={theme}
            onThemeChange={setTheme}
            fontSize={fontSize}
            onFontSizeChange={setFontSize}
          />
        )
      }
    />
  )
}
