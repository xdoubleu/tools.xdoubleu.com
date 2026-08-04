import { useCallback, useMemo } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { ConnectError, Code } from '@connectrpc/connect'
import { createServiceClient } from '@/lib/client'
import { sha256Hex } from '@/lib/books/checksum'
import { createRateLimiter } from '@/lib/books/rateLimiter'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import {
  LibraryService,
  CreateBookRequestSchema,
  UpdateBookStatusRequestSchema,
  UpdateProgressRequestSchema
} from '@/lib/gen/books/v1/library_pb'
import { BookFilesService } from '@/lib/gen/books/v1/files_pb'
import { KoboService } from '@/lib/gen/books/v1/kobo_pb'
import { CatalogService } from '@/lib/gen/books/v1/catalog_pb'
import type {
  GetLibraryResponse,
  GetBooksProgressResponse,
  SearchLibraryResponse,
  SearchExternalResponse,
  GetExternalBookResponse,
  GetBookContentResponse,
  Book
} from '@/lib/gen/books/v1/library_pb'
import type { GetKEPUBStatusResponse, GetBookFileResponse } from '@/lib/gen/books/v1/files_pb'
import type { ListKoboDevicesResponse, GetKoboDeviceLogsResponse } from '@/lib/gen/books/v1/kobo_pb'
import type {
  FindDuplicatesResponse,
  ListResyncProposalsResponse,
  GetBookSourcesResponse,
  GetSourceStatsResponse,
  ListBooksInExactSourcesResponse
} from '@/lib/gen/books/v1/catalog_pb'

export type CreateBookInput = MessageInitShape<typeof CreateBookRequestSchema>
export type UpdateBookStatusInput = MessageInitShape<typeof UpdateBookStatusRequestSchema>
export type UpdateProgressInput = MessageInitShape<typeof UpdateProgressRequestSchema>

export function useLibrary() {
  const client = createServiceClient(LibraryService)
  return useSWR<GetLibraryResponse, Error>(swrKeys.books, () => client.getLibrary({}))
}

export function useBooksProgress(dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(LibraryService)
  const key = dateStart || dateEnd ? swrKeys.booksProgress(dateStart, dateEnd) : null
  return useSWR<GetBooksProgressResponse, Error>(key, () =>
    client.getBooksProgress({ dateStart, dateEnd })
  )
}

export function useSearchLibrary() {
  const client = useMemo(() => createServiceClient(LibraryService), [])
  return useCallback(
    (query: string) =>
      // A type-ahead widget (BookSearchBar), not a listing page — bounding
      // the match count is enough, no "load more" UI needed here.
      client
        .searchLibrary({ query, limit: DEFAULT_PAGE_SIZE })
        .then((r: SearchLibraryResponse) => r),
    [client]
  )
}

export function useSearchExternal() {
  const client = useMemo(() => createServiceClient(LibraryService), [])
  return useCallback(
    (query: string) => client.searchExternal({ query }).then((r: SearchExternalResponse) => r),
    [client]
  )
}

// useExternalBook fetches a single not-in-library book from a provider (e.g.
// Hardcover) for the external book detail page. Null args disable the
// fetch (route params not resolved yet).
export function useExternalBook(provider: string | null, providerId: string | null) {
  const client = createServiceClient(LibraryService)
  return useSWR<GetExternalBookResponse, Error>(
    provider && providerId ? swrKeys.externalBook(provider, providerId) : null,
    () => client.getExternalBook({ provider: provider!, providerId: providerId! })
  )
}

export function useCreateBook() {
  const client = createServiceClient(LibraryService)
  return (req: CreateBookInput) => client.createBook(req)
}

export function useImportBooks() {
  const client = createServiceClient(CatalogService)
  return (csvData: string) => {
    const encoder = new TextEncoder()
    return client.importBooks({ csvData: encoder.encode(csvData) })
  }
}

export function useUpdateBookStatus() {
  const client = createServiceClient(LibraryService)
  return (req: UpdateBookStatusInput) => client.updateBookStatus(req)
}

export function useToggleTag() {
  const client = createServiceClient(LibraryService)
  return (bookId: string, tag: string) => client.toggleTag({ bookId, tag })
}

export function useUpdateFinishedAt() {
  const client = createServiceClient(LibraryService)
  return (bookId: string, finishedAt: string[]) => client.updateFinishedAt({ bookId, finishedAt })
}

export function useUpdateProgress() {
  const client = createServiceClient(LibraryService)
  return (req: UpdateProgressInput) => client.updateProgress(req)
}

export function useRemoveBook() {
  const client = createServiceClient(LibraryService)
  return (bookId: string) => client.removeBook({ bookId })
}

export type UploadBookFileResult = {
  matchedExisting: boolean
  recognizedTitle: string
}

export type UploadBookFileOverride = {
  titleOverride?: string
  authorOverride?: string
}

// Paces requests below the API's global per-IP limiter (10 req/s, burst 30,
// shared across all site traffic) so bulk imports don't outrun it — see
// issue #824/#828/#833. withRateLimitRetry's backoff below stays as a
// secondary safety net for whatever it doesn't fully absorb.
const uploadLimiter = createRateLimiter(3, 6)

// withRateLimitRetry retries a Connect RPC on Code.ResourceExhausted (the
// mapping of the API's HTTP 429), which BulkBookUploader's concurrent
// create+finalize calls can trip under a large import (issue #824) even
// though each is a legitimate request. Backs off instead of failing the file.
async function withRateLimitRetry<T>(call: () => Promise<T>): Promise<T> {
  const maxAttempts = 4
  for (let attempt = 0; ; attempt++) {
    await uploadLimiter()
    try {
      return await call()
    } catch (err) {
      const limited = err instanceof ConnectError && err.code === Code.ResourceExhausted
      if (!limited || attempt === maxAttempts - 1) throw err
      await new Promise((resolve) => setTimeout(resolve, 200 * 2 ** attempt))
    }
  }
}

export function useUploadBookFile() {
  const client = createServiceClient(BookFilesService)
  return async (file: File, override?: UploadBookFileOverride): Promise<UploadBookFileResult> => {
    // 0. Compute file hash so the server can skip a duplicate upload.
    const checksum = await sha256Hex(file)

    // 1. Ask the server whether the content already exists.
    //    When alreadyExists is true the server already has the blob, so the
    //    client skips the PUT and goes straight to Finalize.
    const { uploadId, url, alreadyExists } = await withRateLimitRetry(() =>
      client.createBookUpload({
        filename: file.name,
        contentType: file.type || 'application/octet-stream',
        size: BigInt(file.size),
        checksum
      })
    )

    if (!alreadyExists) {
      // 2. PUT the file directly to R2, bypassing the API server and DO ingress.
      const putResp = await fetch(url, {
        method: 'PUT',
        body: file,
        headers: { 'Content-Type': file.type || 'application/octet-stream' }
      })
      if (!putResp.ok) {
        throw new Error(`Upload to storage failed (${putResp.status})`)
      }
    }

    // 3. Tell the API to validate, register, and recognise the file.
    //    On FailedPrecondition the blob disappeared between Create and Finalize
    //    (race condition); retry the full flow once without the checksum shortcut
    //    so the client uploads the bytes this time.
    try {
      const result = await withRateLimitRetry(() =>
        client.finalizeBookUpload({
          uploadId,
          filename: file.name,
          contentType: file.type || 'application/octet-stream',
          checksum,
          titleOverride: override?.titleOverride ?? '',
          authorOverride: override?.authorOverride ?? ''
        })
      )
      return {
        matchedExisting: result.matchedExisting,
        recognizedTitle: result.recognizedTitle
      }
    } catch (err) {
      if (alreadyExists && err instanceof ConnectError && err.code === Code.FailedPrecondition) {
        // The canonical blob was deleted between Create and Finalize; upload now.
        const retry = await withRateLimitRetry(() =>
          client.createBookUpload({
            filename: file.name,
            contentType: file.type || 'application/octet-stream',
            size: BigInt(file.size)
            // No checksum: force a fresh upload URL.
          })
        )
        const putResp = await fetch(retry.url, {
          method: 'PUT',
          body: file,
          headers: { 'Content-Type': file.type || 'application/octet-stream' }
        })
        if (!putResp.ok) {
          throw new Error(`Upload to storage failed on retry (${putResp.status})`, {
            cause: err
          })
        }
        const retryResult = await withRateLimitRetry(() =>
          client.finalizeBookUpload({
            uploadId: retry.uploadId,
            filename: file.name,
            contentType: file.type || 'application/octet-stream',
            checksum,
            titleOverride: override?.titleOverride ?? '',
            authorOverride: override?.authorOverride ?? ''
          })
        )
        return {
          matchedExisting: retryResult.matchedExisting,
          recognizedTitle: retryResult.recognizedTitle
        }
      } else {
        throw err
      }
    }
  }
}

export function useEnableKoboSync() {
  const client = createServiceClient(KoboService)
  return (bookId: string) => client.enableKoboSync({ bookId })
}

export function useRequestKEPUBConversion() {
  const client = createServiceClient(BookFilesService)
  return (bookId: string) => client.requestKEPUBConversion({ bookId })
}

export function useRegisterKoboDevice() {
  const client = createServiceClient(KoboService)
  return (name: string, serial: string) => client.registerKoboDevice({ name, serial })
}

export function useListKoboDevices() {
  const client = createServiceClient(KoboService)
  return useSWR<ListKoboDevicesResponse, Error>(swrKeys.koboDevices, () =>
    client.listKoboDevices({})
  )
}

export function useDisconnectKoboDevice() {
  const client = createServiceClient(KoboService)
  return (id: string) => client.disconnectKoboDevice({ id })
}

export function useSetKoboDeviceLogging() {
  const client = createServiceClient(KoboService)
  return (id: string, enabled: boolean) => client.setKoboDeviceLogging({ id, enabled })
}

export function useKoboDeviceLogs(id: string, enabled: boolean) {
  const client = createServiceClient(KoboService)
  return useSWR<GetKoboDeviceLogsResponse, Error>(enabled ? swrKeys.koboDeviceLogs(id) : null, () =>
    client.getKoboDeviceLogs({ id })
  )
}

export function useClearKoboDeviceLogs() {
  const client = createServiceClient(KoboService)
  return (id: string) => client.clearKoboDeviceLogs({ id })
}

export function useStartResync() {
  const client = createServiceClient(CatalogService)
  return (force = false) => client.startResync({ force })
}

export function useCancelResync() {
  const client = createServiceClient(CatalogService)
  return () => client.cancelResync({})
}

export function useFindDuplicates() {
  const client = createServiceClient(CatalogService)
  return useSWR<FindDuplicatesResponse, Error>(swrKeys.bookDuplicates, () =>
    client.findDuplicates({})
  )
}

export interface MergeBooksOptions {
  resolvedMetadata?: Book
  resolvedCoverSourceBookId?: string
  resolvedStatus?: string
}

export function useMergeBooks() {
  const client = createServiceClient(CatalogService)
  return (winnerBookId: string, loserBookIds: string[], options?: MergeBooksOptions) =>
    client.mergeBooks({
      winnerBookId,
      loserBookIds,
      resolvedMetadata: options?.resolvedMetadata,
      resolvedCoverSourceBookId: options?.resolvedCoverSourceBookId,
      resolvedStatus: options?.resolvedStatus
    })
}

export function useResyncProposals() {
  const client = createServiceClient(CatalogService)
  return useSWR<ListResyncProposalsResponse, Error>(swrKeys.resyncProposals, () =>
    client.listResyncProposals({})
  )
}

export function useApplyResyncChoice() {
  const client = useMemo(() => createServiceClient(CatalogService), [])
  return useCallback(
    (bookId: string, source: string) => client.applyResyncChoice({ bookId, source }),
    [client]
  )
}

// SourceSearchOverride carries admin-tweaked title/author search terms for
// the live source fetch, used when the stored fields find no match.
export interface SourceSearchOverride {
  title: string
  author: string
}

// useBookSources live-fetches one book's candidate sources for the book-page
// admin sync control. enabled gates the fetch behind a user action (the
// live fetch hits every configured provider, so it shouldn't run on mount).
export function useBookSources(bookId: string, enabled: boolean, override?: SourceSearchOverride) {
  const client = createServiceClient(CatalogService)
  return useSWR<GetBookSourcesResponse, Error>(
    enabled ? swrKeys.bookSources(bookId, override?.title ?? '', override?.author ?? '') : null,
    () =>
      client.getBookSources({
        bookId,
        overrideTitle: override?.title || undefined,
        overrideAuthor: override?.author || undefined
      })
  )
}

export function useApplyBookSource() {
  const client = useMemo(() => createServiceClient(CatalogService), [])
  return useCallback(
    (bookId: string, source: string, index: number, override?: SourceSearchOverride) =>
      client.applyBookSource({
        bookId,
        source,
        index,
        overrideTitle: override?.title || undefined,
        overrideAuthor: override?.author || undefined
      }),
    [client]
  )
}

export function useSourceStats() {
  const client = createServiceClient(CatalogService)
  return useSWR<GetSourceStatsResponse, Error>(swrKeys.bookSourceStats, () =>
    client.getSourceStats({})
  )
}

export function useBooksInExactSources(sources: string[] | null) {
  const client = createServiceClient(CatalogService)
  return useSWR<ListBooksInExactSourcesResponse, Error>(
    sources && sources.length > 0 ? swrKeys.bookBooksInExactSources(sources) : null,
    () => client.listBooksInExactSources({ sources: sources! })
  )
}

export function useSetBookISBN() {
  const client = useMemo(() => createServiceClient(CatalogService), [])
  return useCallback(
    (bookId: string, isbn13: string) => client.setBookISBN({ bookId, isbn13 }),
    [client]
  )
}

export function useKEPUBStatus(bookId: string | null) {
  const client = createServiceClient(BookFilesService)
  return useSWR<GetKEPUBStatusResponse, Error>(
    bookId ? swrKeys.kepubStatus(bookId) : null,
    () => client.getKEPUBStatus({ bookId: bookId! }),
    { refreshInterval: (data) => (data?.kepubStatus === 'converting' ? 2000 : 0) }
  )
}

export function useGetBookFile(bookId: string | null, format: string | null) {
  const client = createServiceClient(BookFilesService)
  return useSWR<GetBookFileResponse, Error>(
    bookId && format ? swrKeys.bookFile(bookId, format) : null,
    () => client.getBookFile({ bookId: bookId!, format: format! })
  )
}

export function useGetBookContent(bookId: string | null) {
  const client = createServiceClient(LibraryService)
  return useSWR<GetBookContentResponse, Error>(bookId ? swrKeys.bookContent(bookId) : null, () =>
    client.getBookContent({ bookId: bookId! })
  )
}

export function useCreateShelf() {
  const client = createServiceClient(LibraryService)
  return (name: string) => client.createShelf({ name })
}

export function useRenameShelf() {
  const client = createServiceClient(LibraryService)
  return (oldName: string, newName: string) => client.renameShelf({ oldName, newName })
}

export function useDeleteShelf() {
  const client = createServiceClient(LibraryService)
  return (name: string, targetName: string) => client.deleteShelf({ name, targetName })
}

export function useRenameTag() {
  const client = createServiceClient(LibraryService)
  return (oldName: string, newName: string) => client.renameTag({ oldName, newName })
}

export function useDeleteTag() {
  const client = createServiceClient(LibraryService)
  return (name: string) => client.deleteTag({ name })
}
