'use client'

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import dynamic from 'next/dynamic'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { ErrorState, LoadingState } from '@/components/ui/states'
import type { ReaderLocation } from '@/components/books/reader/BookReader'
import { useLibrary } from '@/hooks/useBooks'
import { useKEPUBConversion } from '@/hooks/useKEPUBConversion'
import { useOfflineBookFile, useStoredBookVersion } from '@/hooks/useOfflineBooks'
import { useReaderChoice } from '@/hooks/useReaderChoice'
import {
  translateReadingPosition,
  useReadingProgressSaver,
  useReadingState
} from '@/hooks/useReadingState'
import { flattenLibrary } from '@/lib/books/bookShelves'
import { readerFileFormat, type ReaderChoice } from '@/lib/books/readerChoice'
import { resumeFromState, type ReaderResume } from '@/lib/books/readerPosition'
import { pickReaderFormat } from '@/lib/books/readerSettings'

// foliate-js drives the DOM directly; never render the reader on the server.
// Stryker disable next-line all: next/dynamic options must stay an object literal.
const BookReader = dynamic(() => import('@/components/books/reader/BookReader'), { ssr: false })

function Fallback({ id, children }: { id: string; children: ReactNode }) {
  return (
    <PageContainer size="narrow" className="space-y-4">
      {children}
      <Button asChild variant="secondary">
        <Link href={`/books/${id}`}>Back to book</Link>
      </Button>
    </PageContainer>
  )
}

/**
 * Opens the book's original EPUB (preferred) or PDF, or its converted KEPUB,
 * at the newest saved position; `?format=` picks one. The file is kept on the
 * device for offline reading. Page changes are saved back, and switching
 * files keeps the position.
 */
export default function ReadBookClient({ id }: { id: string }) {
  const router = useRouter()
  const requestedFormat = useSearchParams().get('format')
  const { data, error, isLoading } = useLibrary()

  const userBook = useMemo(() => {
    if (!data?.library) return null
    return flattenLibrary(data.library).find((ub) => ub.id === id) ?? null
  }, [data, id])

  const bookId = userBook?.bookId ?? null
  const [choice, choose] = useReaderChoice(bookId, requestedFormat)
  const original = userBook ? pickReaderFormat(userBook.formats, requestedFormat) : null
  const format = original && choice ? readerFileFormat(original, choice) : null
  const version = (format && userBook?.fileVersions[format]) || ''
  const conversion = useKEPUBConversion(format === 'kepub' ? bookId : null)
  // A stored KEPUB opens without waiting for the conversion when it is
  // current, or when the conversion can't be confirmed (e.g. offline).
  const storedKEPUB = useStoredBookVersion(format === 'kepub' ? bookId : null, 'kepub')
  const kepubOpenable =
    conversion === 'ready' ||
    (typeof storedKEPUB === 'string' && (storedKEPUB === version || conversion === 'failed'))
  const fileFormat = format === 'kepub' && !kepubOpenable ? null : format
  const { file, error: fileError } = useOfflineBookFile(
    fileFormat ? bookId : null,
    fileFormat,
    version
  )

  // Resume from a read made after mounting: the SWR cache may predate this
  // session's saves or another device's. A failed read falls back to the cache.
  const readingState = useReadingState(bookId)
  const refreshReadingState = readingState.mutate
  const cachedReadingState = useRef(readingState.data)
  useEffect(() => {
    cachedReadingState.current = readingState.data
  }, [readingState.data])
  const [resume, setResume] = useState<ReaderResume | null>(null)
  useEffect(() => {
    if (!bookId) return
    let cancelled = false
    void refreshReadingState()
      .catch(() => cachedReadingState.current)
      .then((fresh) => {
        if (!cancelled) setResume((prev) => prev ?? resumeFromState(fresh?.state))
      })
    return () => {
      cancelled = true
    }
  }, [bookId, refreshReadingState])
  const saveProgress = useReadingProgressSaver(bookId)

  // A switch reopens at the last page read. An EPUB position fits its KEPUB
  // exactly; a PDF-sourced book's position is translated by the server, and
  // falls back to the percent when that fails (e.g. offline).
  const lastLocation = useRef<ReaderLocation | null>(null)
  const latestSwitch = useRef<object | null>(null)
  const onRelocate = (location: ReaderLocation) => {
    lastLocation.current = location
    saveProgress(location)
  }
  const switchTo = (next: ReaderChoice) => {
    const at = lastLocation.current
    const thisSwitch = {}
    latestSwitch.current = thisSwitch
    const position = at?.position
    if (at && original === 'pdf' && bookId && position) {
      const percent = at.fraction * 100
      setResume(null)
      void translateReadingPosition(bookId, position)
        .then((translated) => resumeFromState({ percent, position: translated }))
        .catch(() => ({ position, percent }))
        .then((r) => {
          if (latestSwitch.current === thisSwitch) setResume(r)
        })
    } else if (at) {
      setResume({ position, percent: at.fraction * 100 })
    }
    choose(next)
  }

  const close = () => {
    // A direct reader entry (card/dashboard) adds a history entry, so go back
    // where the book was opened from; a deep link falls back to the page.
    if (window.history.length > 1) router.back()
    else router.push(`/books/${id}`)
  }

  if (error && !userBook) {
    return (
      <Fallback id={id}>
        <ErrorState what="book" />
      </Fallback>
    )
  }
  if (isLoading && !userBook) {
    return (
      <Fallback id={id}>
        <LoadingState label="book" />
      </Fallback>
    )
  }
  if (!userBook) {
    return (
      <Fallback id={id}>
        <p className="text-muted">Book not found.</p>
      </Fallback>
    )
  }
  if (!original) {
    return (
      <Fallback id={id}>
        <p className="text-muted">This book has no EPUB or PDF file.</p>
      </Fallback>
    )
  }
  if (format === 'kepub' && !kepubOpenable) {
    if (storedKEPUB === undefined) {
      return (
        <Fallback id={id}>
          <LoadingState label="book" />
        </Fallback>
      )
    }
    return (
      <Fallback id={id}>
        <div className="space-y-3">
          {conversion === 'failed' ? (
            <Alert tone="danger">Conversion failed.</Alert>
          ) : (
            <Alert tone="info">Converting… this may take a moment.</Alert>
          )}
          <Button type="button" variant="secondary" onClick={() => switchTo('original')}>
            Read original
          </Button>
        </div>
      </Fallback>
    )
  }
  if (!file && fileError) {
    return (
      <Fallback id={id}>
        <ErrorState what="book file" />
      </Fallback>
    )
  }
  if (!file || !resume) {
    return (
      <Fallback id={id}>
        <LoadingState label="book" />
      </Fallback>
    )
  }

  return (
    <BookReader
      key={format}
      file={file}
      title={userBook.book?.title ?? 'Book'}
      onClose={close}
      onRelocate={onRelocate}
      initialPosition={resume}
      format={{ value: choice!, original, onChange: switchTo }}
    />
  )
}
