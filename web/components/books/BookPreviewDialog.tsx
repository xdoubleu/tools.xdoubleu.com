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

  // For on-demand KEPUB conversion: trigger once per dialog open.
  const requestKEPUBConversion = useRequestKEPUBConversion()
  const triggeredRef = useRef(false)
  useEffect(() => {
    if (!open || !isKepub || triggeredRef.current) return
    triggeredRef.current = true
    // Re-fetch status after the request resolves: the response above kicks off
    // conversion server-side without updating this dialog's SWR cache, so
    // without this the poll below can see a stale "" status, stop polling, and
    // never notice the conversion complete (issue #393).
    void requestKEPUBConversion(bookId).then(() => mutate(swrKeys.kepubStatus(bookId)))
  }, [open, isKepub, bookId, requestKEPUBConversion])

  // Reset the trigger guard when the dialog closes.
  useEffect(() => {
    if (!open) triggeredRef.current = false
  }, [open])

  // Poll KEPUB status while converting (only when format is kepub).
  const { data: kepubStatusData } = useKEPUBStatus(isKepub && open ? bookId : null)
  const kepubStatus = kepubStatusData?.kepubStatus ?? ''
  const kepubReady = kepubStatus === 'ready'
  const kepubFailed = kepubStatus === 'failed'

  // Fetch the presigned file URL:
  // - pdf/epub: fetch as soon as the dialog opens.
  // - kepub: only fetch once the KEPUB conversion is ready.
  const fileBookId = open ? bookId : null
  const fileFormat = isKepub ? (kepubReady ? 'kepub' : null) : open ? format : null
  const { data, error } = useGetBookFile(fileBookId, fileFormat)

  // react-reader/epub.js has no cancellation for its in-flight archive fetch/parse chain:
  // closing the dialog while it's still loading destroys the underlying book but the pending
  // promise chain runs on regardless and throws reading the now-cleared state, surfacing as an
  // unhandled rejection well after unmount (issue #1158) rather than a catchable render error.
  // Swallow only these two known-benign signatures; keep the guard alive briefly past unmount
  // since the rejection can land after this effect's own cleanup has already run.
  useEffect(() => {
    if (!data || isPDF) return
    const isKnownReaderRace = (message: string) =>
      message === 'Connection closed.' || message.includes("evaluating 'this.book.package'")
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
      <DialogContent
        className={isPDF ? 'max-w-4xl h-[85vh] flex flex-col' : 'max-w-2xl h-[85vh] flex flex-col'}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogClose aria-label="Close preview">X</DialogClose>
        </DialogHeader>

        <div className="flex-1 overflow-hidden rounded-xl">
          {/* KEPUB-specific states */}
          {isKepub && kepubFailed && (
            <p className="text-sm text-danger p-4">Conversion failed. Cannot preview EPUB.</p>
          )}
          {isKepub && !kepubFailed && !kepubReady && (
            <p className="text-sm text-muted p-4">Converting… this may take a moment.</p>
          )}

          {/* Generic fetch error (pdf/epub paths) */}
          {!isKepub && error && <p className="text-sm text-danger p-4">Failed to load preview.</p>}

          {/* Loading state for pdf/epub while URL is being fetched */}
          {!isKepub && !data && !error && (
            <p className="text-sm text-muted p-4">Loading preview…</p>
          )}

          {/* KEPUB fetch error once ready */}
          {isKepub && kepubReady && error && (
            <p className="text-sm text-danger p-4">Failed to load preview.</p>
          )}

          {data && isPDF && (
            <iframe src={data.url} title={`Preview: ${title}`} className="w-full h-full border-0" />
          )}

          {data && !isPDF && (
            // react-reader fetches the EPUB bytes directly from the presigned URL.
            // The R2 bucket must allow GET requests from this site's origin via a
            // CORS rule (deploy step) — without it the EPUB will fail to load.
            //
            // epub.js determineType() branches on the URL file extension: only
            // ".epub" triggers archive mode; ".kepub" (and anything else) falls
            // through to directory mode, which probes META-INF/container.xml on a
            // sub-path not covered by the presign signature. Force archive mode so
            // both native EPUB and converted KEPUB are fetched as a single zip.
            <ReactReader
              url={data.url}
              title={title}
              epubInitOptions={{ openAs: 'epub' }}
              // location/locationChanged are required by ReactReader's props
              location={null}
              locationChanged={() => {}}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
