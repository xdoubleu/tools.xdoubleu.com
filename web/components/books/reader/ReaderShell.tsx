import type { ReactNode, RefObject } from 'react'
import { Button } from '@/components/ui/button'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { readerBackground, type ReaderTheme } from '@/lib/books/readerSettings'

export type ReaderStatus = 'loading' | 'ready' | 'error'

interface ReaderShellProps {
  title: string
  onClose: () => void
  /** Header controls, shown once the book is open. */
  controls?: ReactNode
  containerRef: RefObject<HTMLDivElement | null>
  status: ReaderStatus
  theme: ReaderTheme
  /** Overall progress, 0..1. */
  fraction: number
  tocLabel?: string
}

/** Full-screen reader chrome: header, the reading area `containerRef` mounts into, progress bar. */
export default function ReaderShell({
  title,
  onClose,
  controls,
  containerRef,
  status,
  theme,
  fraction,
  tocLabel
}: ReaderShellProps) {
  const percent = Math.round(fraction * 100)

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
        {controls}
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
          <span className="min-w-0 truncate">{tocLabel}</span>
          <span className="shrink-0 tabular-nums">{percent}%</span>
        </div>
      </div>
    </div>
  )
}
