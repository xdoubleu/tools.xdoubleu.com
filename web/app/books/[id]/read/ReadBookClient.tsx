'use client'

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import dynamic from 'next/dynamic'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { mutate } from 'swr'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { ErrorState, LoadingState } from '@/components/ui/states'
import type { ReaderLocation } from '@/components/books/reader/BookReader'
import { useGetBookFile, useLibrary } from '@/hooks/useBooks'
import { useKEPUBConversion } from '@/hooks/useKEPUBConversion'
import { useReaderChoice } from '@/hooks/useReaderChoice'
import { useReadingProgressSaver, useReadingState } from '@/hooks/useReadingState'
import { flattenLibrary } from '@/lib/books/bookShelves'
import { readerFileFormat, type ReaderChoice } from '@/lib/books/readerChoice'
import { resumeFromState, type ReaderResume } from '@/lib/books/readerPosition'
import { pickReaderFormat } from '@/lib/books/readerSettings'
import { swrKeys } from '@/lib/swrKeys'

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
 * at the newest saved position; `?format=` picks one. Page changes are saved
 * back, and switching files keeps the position.
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
  const conversion = useKEPUBConversion(format === 'kepub' ? bookId : null)
  const fileFormat = format === 'kepub' && conversion !== 'ready' ? null : format
  const file = useGetBookFile(fileFormat ? bookId : null, fileFormat)

  // Pin the first fresh URL: a refetch mints a new presigned URL, which would
  // reopen the book at the start, and a cached one may have expired.
  const [pinned, setPinned] = useState<{ format: string; url: string } | null>(null)
  const url = pinned && pinned.format === format ? pinned.url : null
  if (!url && format && file.data && !file.isValidating) {
    setPinned({ format, url: file.data.url })
  }

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
  // exactly; a PDF page doesn't, so the reader falls back to the percent.
  const lastLocation = useRef<ReaderLocation | null>(null)
  const onRelocate = (location: ReaderLocation) => {
    lastLocation.current = location
    saveProgress(location)
  }
  const switchTo = (next: ReaderChoice) => {
    const at = lastLocation.current
    if (at) setResume({ position: at.position, percent: at.fraction * 100 })
    // Drop a cached URL for the other file; it may have expired.
    void mutate(swrKeys.bookFile(bookId!, readerFileFormat(original!, next)), undefined, {
      revalidate: false
    })
    choose(next)
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
  if (format === 'kepub' && conversion !== 'ready') {
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
  if (!url && file.error) {
    return (
      <Fallback id={id}>
        <ErrorState what="book file" />
      </Fallback>
    )
  }
  if (!url || !resume) {
    return (
      <Fallback id={id}>
        <LoadingState label="book" />
      </Fallback>
    )
  }

  return (
    <BookReader
      key={url}
      url={url}
      title={userBook.book?.title ?? 'Book'}
      onClose={() => router.push(`/books/${id}`)}
      onRelocate={onRelocate}
      initialPosition={resume}
      format={{ value: choice!, original, onChange: switchTo }}
    />
  )
}
