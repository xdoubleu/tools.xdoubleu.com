'use client'

import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import dynamic from 'next/dynamic'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { useLibrary } from '@/hooks/useBooks'
import { useOfflineBookFile } from '@/hooks/useOfflineBooks'
import { useReadingProgressSaver, useReadingState } from '@/hooks/useReadingState'
import { flattenLibrary } from '@/lib/books/bookShelves'
import { resumeFromState, type ReaderLocation, type ReaderResume } from '@/lib/books/readerPosition'
import { pickReaderFormat } from '@/lib/books/readerSettings'
import { isWebPubWarm } from '@/lib/books/webpubCache'

// Readium builds iframes and reads sections from the API; client-only too.
// Stryker disable next-line all: next/dynamic options must stay an object literal.
const ReadiumBookReader = dynamic(() => import('@/components/books/reader/ReadiumBookReader'), {
  ssr: false
})

// pdf.js draws to a canvas and runs a worker; client-only too.
// Stryker disable next-line all: next/dynamic options must stay an object literal.
const PdfBookReader = dynamic(() => import('@/components/books/reader/PdfBookReader'), {
  ssr: false
})

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
 * Opens the book's original EPUB (preferred, in Readium) or PDF at the newest
 * saved position; `?format=` picks one. The file is kept on the device for
 * offline reading. Page changes are saved back.
 */
export default function ReadBookClient({ id }: { id: string }) {
  const router = useRouter()
  const [openedOnline] = useState(() => navigator.onLine)
  // Offline, Readium reads the copy kept by the offline sync, if there is one.
  const [webPubCached, setWebPubCached] = useState<boolean | null>(null)
  const requestedFormat = useSearchParams().get('format')
  const { data, error, isLoading } = useLibrary()

  const userBook = useMemo(() => {
    if (!data?.library) return null
    return flattenLibrary(data.library).find((ub) => ub.id === id) ?? null
  }, [data, id])

  const bookId = userBook?.bookId ?? null
  useEffect(() => {
    if (openedOnline || !bookId) return
    void isWebPubWarm(bookId).then(setWebPubCached, () => setWebPubCached(false))
  }, [openedOnline, bookId])
  const format = userBook ? pickReaderFormat(userBook.formats, requestedFormat) : null
  const version = (format && userBook?.fileVersions[format]) || ''
  // Readium reads an EPUB from the API (or its offline copy); only a PDF is a file.
  const { file, error: fileError } = useOfflineBookFile(
    format === 'pdf' ? bookId : null,
    format,
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

  const onRelocate = (location: ReaderLocation) => saveProgress(location)

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
  if (!format) {
    return (
      <Fallback id={id}>
        <p className="text-muted">This book has no EPUB or PDF file.</p>
      </Fallback>
    )
  }
  if (format === 'epub') {
    if (!openedOnline && webPubCached === null) {
      return (
        <Fallback id={id}>
          <LoadingState label="book" />
        </Fallback>
      )
    }
    if (!openedOnline && !webPubCached) {
      return (
        <Fallback id={id}>
          <p className="text-muted">This book isn&apos;t available offline yet.</p>
        </Fallback>
      )
    }
    return resume ? (
      <ReadiumBookReader
        key={bookId}
        bookId={userBook.bookId}
        title={userBook.book?.title ?? 'Book'}
        onClose={close}
        onRelocate={onRelocate}
        initialPosition={resume}
      />
    ) : (
      <Fallback id={id}>
        <LoadingState label="book" />
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
    <PdfBookReader
      key={bookId}
      file={file}
      title={userBook.book?.title ?? 'Book'}
      onClose={close}
      onRelocate={onRelocate}
      initialPosition={resume}
    />
  )
}
