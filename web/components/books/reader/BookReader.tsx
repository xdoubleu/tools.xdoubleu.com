'use client'

import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { useLocalStorage } from '@/hooks/useLocalStorage'
import {
  READER_FONT_SIZE_DEFAULT,
  fixedLayoutFilter,
  readerBackground,
  readerStyles,
  type ReaderTheme
} from '@/lib/books/readerSettings'
import type { ReaderChoiceControl } from '@/lib/books/readerChoice'
import type { ReaderResume } from '@/lib/books/readerPosition'
import ReaderControls from './ReaderControls'
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
  /** Offers switching between the original file and its converted KEPUB. */
  format?: ReaderChoiceControl
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
  initialPosition,
  format
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

  const percent = Math.round((location?.fraction ?? 0) * 100)

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-bg text-fg">
      <div className="flex shrink-0 items-center gap-2 border-b border-border bg-card px-1 pt-[var(--inset-top)]">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Close reader"
          onClick={onClose}
        >
          <span aria-hidden="true" className="text-xl">
            ‹
          </span>
        </Button>
        <p className="min-w-0 flex-1 truncate text-sm font-medium">{title}</p>
        {view && (
          <ReaderControls
            view={view}
            currentHref={location?.tocHref}
            theme={theme}
            onThemeChange={setTheme}
            fontSize={fontSize}
            onFontSizeChange={setFontSize}
            format={format}
          />
        )}
      </div>

      <div
        className="relative min-h-0 flex-1"
        aria-busy={status !== 'ready'}
        style={{ background: readerBackground(theme) }}
      >
        <div ref={containerRef} className="absolute inset-0" />
        {status === 'loading' && (
          <LoadingState label="book" className="absolute inset-x-0 top-1/3 text-center" />
        )}
        {status === 'error' && (
          <ErrorState what="book" className="absolute inset-x-0 top-1/3 text-center" />
        )}
      </div>

      <div className="shrink-0 border-t border-border bg-card px-4 pt-2 pb-[calc(0.5rem+env(safe-area-inset-bottom))]">
        <div
          className="h-1 w-full overflow-hidden rounded-sm bg-surface"
          role="progressbar"
          aria-label="Reading progress"
          aria-valuenow={percent}
          aria-valuemin={0}
          aria-valuemax={100}
        >
          <div className="h-full bg-accent" style={{ width: `${percent}%` }} />
        </div>
        <div className="mt-1 flex justify-between gap-3 text-xs text-muted">
          <span className="min-w-0 truncate">{location?.tocLabel}</span>
          <span className="shrink-0 tabular-nums">{percent}%</span>
        </div>
      </div>
    </div>
  )
}
