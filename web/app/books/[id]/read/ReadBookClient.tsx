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
import { resumeFromState, type ReaderResume } from '@/lib/books/readerPosition'
import { pickReaderFormat } from '@/lib/books/readerSettings'

// foliate-js drives the DOM directly; never render the reader on the server.
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
 * Opens the book's original EPUB (preferred) or PDF at the newest saved
 * position; `?format=` picks one. The file is kept on the device for offline
 * reading. Page changes are saved back.
 */
export default function ReadBookClient({ id }: { id: string }) {
  const router = useRouter()
  const requestedFormat = useSearchParams().get('format')
  const { data, error, isLoading } = useLibrary()

  const userBook = useMemo(() => {
    if (!data?.library) return null
    return flattenLibrary(data.library).find((ub) => ub.id === id) ?? null
  }, [data, id])

  const format = userBook ? pickReaderFormat(userBook.formats, requestedFormat) : null
  const { file, error: fileError } = useOfflineBookFile(
    format ? userBook!.bookId : null,
    format,
    (format && userBook?.fileVersions[format]) || ''
  )

  // Resume from a read made after mounting: the SWR cache may predate this
  // session's saves or another device's. A failed read falls back to the cache.
  const bookId = userBook?.bookId ?? null
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
      onClose={() => router.push(`/books/${id}`)}
      onRelocate={saveProgress}
      initialPosition={resume}
    />
  )
}
