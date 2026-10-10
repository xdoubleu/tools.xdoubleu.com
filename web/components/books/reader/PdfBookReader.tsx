'use client'

import { useEffect, useRef, useState } from 'react'
import type { PDFDocumentProxy } from 'pdfjs-dist'
import { useLocalStorage } from '@/hooks/useLocalStorage'
import { openPdf, pageOf, pdfOutline, renderPage, tocLabelForPage } from '@/lib/books/pdf'
import type { ReaderLocation, ReaderResume, ReaderTocItem } from '@/lib/books/readerPosition'
import {
  READER_FONT_SIZE_DEFAULT,
  fixedLayoutFilter,
  swipeDirection,
  tapDirection,
  type PageTurn,
  type ReaderTheme
} from '@/lib/books/readerSettings'
import ReaderControls from './ReaderControls'
import ReaderShell, { type ReaderStatus } from './ReaderShell'

interface PdfBookReaderProps {
  /** The PDF, or a URL to fetch it from. */
  file: Blob | string
  title: string
  onClose: () => void
  /** Fires on every page change after the book opens. */
  onRelocate?: (location: ReaderLocation) => void
  /** Where to open; read once. Defaults to the first page. */
  initialPosition?: ReaderResume
}

function appTheme(): ReaderTheme {
  return document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'
}

function fractionOf(page: number, pages: number): number {
  return pages > 1 ? (page - 1) / (pages - 1) : 0
}

/** The page to open on: a stored page, else the stored percent. */
function startPage(pages: number, resume: ReaderResume | undefined): number {
  const stored = resume?.position
  if (stored && 'page' in stored && stored.page >= 1 && stored.page <= pages) return stored.page
  const percent = resume?.percent ?? 0
  return percent > 0 ? Math.round((percent / 100) * (pages - 1)) + 1 : 1
}

/** Full-screen PDF reader: one page at a time on pdf.js, with tap zones, swipe and contents. */
export default function PdfBookReader({
  file,
  title,
  onClose,
  onRelocate,
  initialPosition
}: PdfBookReaderProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [pdf, setPdf] = useState<PDFDocumentProxy | null>(null)
  const [toc, setToc] = useState<ReaderTocItem[]>([])
  const [page, setPage] = useState(1)
  const [size, setSize] = useState({ width: 0, height: 0 })
  const [status, setStatus] = useState<ReaderStatus>('loading')
  const [storedTheme, setTheme] = useLocalStorage<ReaderTheme | null>('books:reader-theme', null)
  const [fontSize, setFontSize] = useLocalStorage(
    'books:reader-font-size',
    READER_FONT_SIZE_DEFAULT
  )
  const [defaultTheme] = useState(appTheme)
  const theme = storedTheme ?? defaultTheme
  const onRelocateRef = useRef(onRelocate)
  const initialPositionRef = useRef(initialPosition)
  useEffect(() => {
    onRelocateRef.current = onRelocate
  }, [onRelocate])

  useEffect(() => {
    let cancelled = false
    let destroy: (() => Promise<void>) | null = null
    void (async () => {
      try {
        const opened = await openPdf(file)
        destroy = opened.destroy
        const doc = opened.pdf
        const outline = await pdfOutline(doc)
        if (cancelled) {
          void opened.destroy()
          return
        }
        setToc(outline)
        setPage(startPage(doc.numPages, initialPositionRef.current))
        setPdf(doc)
        setStatus('ready')
      } catch {
        if (!cancelled) setStatus('error')
      }
    })()
    return () => {
      cancelled = true
      void destroy?.()
    }
  }, [file])

  useEffect(() => {
    const el = containerRef.current!
    const observer = new ResizeObserver(([entry]) => {
      if (entry) setSize({ width: entry.contentRect.width, height: entry.contentRect.height })
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    if (!pdf || !size.width || !size.height) return
    const abort = new AbortController()
    renderPage(pdf, page, canvasRef.current!, size.width, size.height, abort.signal).catch(() =>
      setStatus('error')
    )
    return () => abort.abort()
  }, [pdf, page, size])

  const pages = pdf?.numPages ?? 0
  const goTo = (next: number) => {
    if (!pdf || next < 1 || next > pages || next === page) return
    setPage(next)
    onRelocateRef.current?.({
      fraction: fractionOf(next, pages),
      section: next - 1,
      tocLabel: tocLabelForPage(toc, next),
      position: { page: next }
    })
  }
  const turn = (direction: PageTurn | null | undefined) => {
    if (direction === 'left') goTo(page - 1)
    if (direction === 'right') goTo(page + 1)
  }
  const turnRef = useRef(turn)
  useEffect(() => {
    turnRef.current = turn
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof Element && e.target.closest('[role="dialog"]')) return
      if (e.key === 'ArrowLeft') turnRef.current('left')
      if (e.key === 'ArrowRight') turnRef.current('right')
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const touchStart = useRef<{ x: number; y: number } | null>(null)

  return (
    <ReaderShell
      title={title}
      onClose={onClose}
      containerRef={containerRef}
      status={status}
      theme={theme}
      fraction={fractionOf(page, pages)}
      tocLabel={tocLabelForPage(toc, page)}
      controls={
        pdf && (
          <ReaderControls
            toc={toc}
            onGoTo={(href) => {
              const target = pageOf(href)
              if (target) goTo(target)
            }}
            reflowable={false}
            currentHref={undefined}
            theme={theme}
            onThemeChange={setTheme}
            fontSize={fontSize}
            onFontSizeChange={setFontSize}
          />
        )
      }
    >
      <div
        data-testid="pdf-surface"
        className="absolute inset-0 flex items-center justify-center"
        onClick={(e) => {
          const area = e.currentTarget.getBoundingClientRect()
          turn(tapDirection(e.clientX, area.left, area.width))
        }}
        onTouchStart={(e) => {
          const t = e.changedTouches[0]
          touchStart.current = t ? { x: t.screenX, y: t.screenY } : null
        }}
        onTouchEnd={(e) => {
          const t = e.changedTouches[0]
          const start = touchStart.current
          touchStart.current = null
          if (start && t) turn(swipeDirection(t.screenX - start.x, t.screenY - start.y))
        }}
      >
        <canvas ref={canvasRef} style={{ filter: fixedLayoutFilter(theme) }} />
      </div>
    </ReaderShell>
  )
}
