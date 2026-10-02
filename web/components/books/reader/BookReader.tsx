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
import ReaderSettingsSheet from './ReaderSettingsSheet'
import ReaderTocDrawer from './ReaderTocDrawer'
import { useFoliateView, type ReaderLocation } from './useFoliateView'

export type { ReaderLocation }

interface BookReaderProps {
  /** Signed URL of the EPUB or PDF. */
  url: string
  title: string
  onClose: () => void
  /** Fires on every page change. */
  onRelocate?: (location: ReaderLocation) => void
}

function appTheme(): ReaderTheme {
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'
}

/** Full-screen foliate-js reader: paginated, with tap zones, swipe, TOC and themes. */
export default function BookReader({ url, title, onClose, onRelocate }: BookReaderProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const { view, status, toc, location } = useFoliateView(containerRef, url, onRelocate)
  const [storedTheme, setTheme] = useLocalStorage<ReaderTheme | null>('books:reader-theme', null)
  const [fontSize, setFontSize] = useLocalStorage(
    'books:reader-font-size',
    READER_FONT_SIZE_DEFAULT
  )
  const [tocOpen, setTocOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
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
        {toc.length > 0 && (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Contents"
            onClick={() => setTocOpen(true)}
          >
            <span aria-hidden="true" className="text-lg">
              ☰
            </span>
          </Button>
        )}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="Reading settings"
          disabled={!view}
          onClick={() => setSettingsOpen(true)}
        >
          <span aria-hidden="true" className="text-sm font-semibold">
            Aa
          </span>
        </Button>
      </div>

      <div className="relative min-h-0 flex-1" style={{ background: readerBackground(theme) }}>
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

      <ReaderTocDrawer
        open={tocOpen}
        onOpenChange={setTocOpen}
        toc={toc}
        currentHref={location?.tocHref}
        onSelect={(href) => {
          setTocOpen(false)
          void view?.goTo(href)
        }}
      />
      <ReaderSettingsSheet
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        theme={theme}
        onThemeChange={setTheme}
        fontSize={fontSize}
        onFontSizeChange={setFontSize}
        reflowable={!view?.isFixedLayout}
      />
    </div>
  )
}
