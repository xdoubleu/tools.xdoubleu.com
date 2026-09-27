'use client'

import { useEffect, useRef } from 'react'
import dynamic from 'next/dynamic'
import { mutate } from 'swr'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { useGetBookFile, useRequestKEPUBConversion, useKEPUBStatus } from '@/hooks/useBooks'
import { swrKeys } from '@/lib/swrKeys'
import { cn } from '@/lib/cn'
import { LoadingState, ErrorState } from '@/components/ui/states'

// react-reader uses the DOM and cannot be server-rendered.
const ReactReader = dynamic(
  () => import('react-reader').then((m) => ({ default: m.ReactReader })),
  { ssr: false }
)

interface BookPreviewDialogProps {
  bookId: string
  format: 'pdf' | 'epub' | 'kepub'
  title: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export default function BookPreviewDialog({
  bookId,
  format,
  title,
  open,
  onOpenChange
}: BookPreviewDialogProps) {
  const isKepub = format === 'kepub'
  const isPDF = format === 'pdf'

  const requestKEPUBConversion = useRequestKEPUBConversion()
  const triggeredRef = useRef(false)
  useEffect(() => {
    if (!open || !isKepub || triggeredRef.current) return
    triggeredRef.current = true
    // Revalidate: the request doesn't update the SWR cache, so polling could
    // stop on a stale "" status.
    void requestKEPUBConversion(bookId).then(() => mutate(swrKeys.kepubStatus(bookId)))
  }, [open, isKepub, bookId, requestKEPUBConversion])

  useEffect(() => {
    if (!open) triggeredRef.current = false
  }, [open])

  const { data: kepubStatusData } = useKEPUBStatus(isKepub && open ? bookId : null)
  const kepubStatus = kepubStatusData?.kepubStatus ?? ''
  const kepubReady = kepubStatus === 'ready'
  const kepubFailed = kepubStatus === 'failed'

  // kepub waits for conversion; pdf/epub fetch on open.
  const fileBookId = open ? bookId : null
  const fileFormat = isKepub ? (kepubReady ? 'kepub' : null) : open ? format : null
  const { data, error } = useGetBookFile(fileBookId, fileFormat)

  // epub.js can't cancel its in-flight load, so closing mid-load throws
  // unhandled rejections after unmount. Swallow only these known-benign
  // messages, and keep the guard alive briefly past cleanup.
  useEffect(() => {
    if (!data || isPDF) return
    const isKnownReaderRace = (message: string) =>
      message === 'Connection closed.' ||
      message.includes("evaluating 'this.book.package'") ||
      // A broken first TOC entry (common in Calibre EPUBs); location={0} covers
      // the initial page, this covers clicking it.
      message === 'No Section Found'
    const onRejection = (event: PromiseRejectionEvent) => {
      const message = event.reason instanceof Error ? event.reason.message : String(event.reason)
      if (isKnownReaderRace(message)) event.preventDefault()
    }
    window.addEventListener('unhandledrejection', onRejection)
    return () => {
      setTimeout(() => window.removeEventListener('unhandledrejection', onRejection), 10000)
    }
  }, [data, isPDF])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn('flex h-[85dvh] flex-col', isPDF ? 'max-w-4xl' : 'max-w-2xl')}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogClose aria-label="Close preview" />
        </DialogHeader>

        <div className="flex-1 overflow-hidden rounded-xl">
          {isKepub && kepubFailed && (
            <p className="text-sm text-danger p-4">Conversion failed. Cannot preview EPUB.</p>
          )}
          {isKepub && !kepubFailed && !kepubReady && (
            <p className="text-sm text-muted p-4">Converting… this may take a moment.</p>
          )}

          {!isKepub && error && <ErrorState what="preview" className="p-4 text-sm" />}

          {!isKepub && !data && !error && <LoadingState label="preview" className="p-4 text-sm" />}

          {isKepub && kepubReady && error && <ErrorState what="preview" className="p-4 text-sm" />}

          {data && isPDF && (
            <iframe src={data.url} title={`Preview: ${title}`} className="w-full h-full border-0" />
          )}

          {data && !isPDF && (
            // Fetched straight from the presigned URL, so R2 needs a CORS rule for
            // this origin. epub.js only uses archive mode for ".epub" URLs; force it
            // for KEPUB too.
            <ReactReader
              url={data.url}
              title={title}
              epubInitOptions={{ openAs: 'epub' }}
              // Spine index 0 always exists, unlike the first TOC entry's target.
              location={0}
              locationChanged={() => {}}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
