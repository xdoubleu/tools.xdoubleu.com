'use client'

import { useEffect, useRef, useState } from 'react'
import { useLocalStorage } from '@/hooks/useLocalStorage'
import {
  READER_FONT_SIZE_DEFAULT,
  fixedLayoutFilter,
  readerStyles,
  type ReaderTheme
} from '@/lib/books/readerSettings'
import type { ReaderResume } from '@/lib/books/readerPosition'
import ReaderControls from './ReaderControls'
import ReaderShell from './ReaderShell'
import { useFoliateView, type ReaderLocation } from './useFoliateView'

export type { ReaderLocation, ReaderResume }

interface BookReaderProps {
  /** The EPUB, KEPUB or PDF, or a URL to fetch it from. */
  file: Blob | string
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

/** Full-screen foliate-js reader: paginated, with tap zones, swipe, TOC and themes. */
export default function BookReader({
  file,
  title,
  onClose,
  onRelocate,
  initialPosition
}: BookReaderProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const { view, status, location } = useFoliateView(containerRef, file, onRelocate, initialPosition)
  const [storedTheme, setTheme] = useLocalStorage<ReaderTheme | null>('books:reader-theme', null)
  const [fontSize, setFontSize] = useLocalStorage(
    'books:reader-font-size',
    READER_FONT_SIZE_DEFAULT
  )
  const [defaultTheme] = useState(appTheme)
  const theme = storedTheme ?? defaultTheme

  useEffect(() => {
    if (!view) return
    if (view.isFixedLayout) view.style.filter = fixedLayoutFilter(theme)
    else view.renderer?.setStyles?.(readerStyles(theme, fontSize))
  }, [view, theme, fontSize])

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
        view && (
          <ReaderControls
            toc={view.book?.toc ?? []}
            onGoTo={(href) => void view.goTo(href)}
            reflowable={!view.isFixedLayout}
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
