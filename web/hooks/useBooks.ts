import { useCallback, useMemo } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { ConnectError, Code } from '@connectrpc/connect'
import { createServiceClient } from '@/lib/client'
import { sha256Hex } from '@/lib/books/checksum'
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
  Book
} from '@/lib/gen/books/v1/library_pb'
import type { GetKEPUBStatusResponse, GetBookFileResponse } from '@/lib/gen/books/v1/files_pb'
import type { ListKoboDevicesResponse } from '@/lib/gen/books/v1/kobo_pb'
import type {
  FindDuplicatesResponse,
  ListResyncProposalsResponse,
  GetBookSourcesResponse
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
    (query: string) => client.searchLibrary({ query }).then((r: SearchLibraryResponse) => r),
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

export function useCompareCSV() {
  const client = createServiceClient(CatalogService)
  return (csvData: string) => {
    const encoder = new TextEncoder()
    return client.compareCSV({ csvData: encoder.encode(csvData) })
  }
}

export function useApplyCSVFix() {
  const client = createServiceClient(CatalogService)
  return (csvData: string, mismatchId: string, difference: string) => {
    const encoder = new TextEncoder()
    return client.applyCSVFix({
      csvData: encoder.encode(csvData),
      mismatchId,
      difference
    })
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

export function useUploadBookFile() {
  const client = createServiceClient(BookFilesService)
  return async (file: File): Promise<UploadBookFileResult> => {
    // 0. Compute file hash so the server can skip a duplicate upload.
    const checksum = await sha256Hex(file)

    // 1. Ask the server whether the content already exists.
    //    When alreadyExists is true the server already has the blob, so the
    //    client skips the PUT and goes straight to Finalize.
    const { uploadId, url, alreadyExists } = await client.createBookUpload({
      filename: file.name,
      contentType: file.type || 'application/octet-stream',
      size: BigInt(file.size),
      checksum
    })

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
      const result = await client.finalizeBookUpload({
        uploadId,
        filename: file.name,
        contentType: file.type || 'application/octet-stream',
        checksum
      })
      return {
        matchedExisting: result.matchedExisting,
        recognizedTitle: result.recognizedTitle
      }
    } catch (err) {
      if (alreadyExists && err instanceof ConnectError && err.code === Code.FailedPrecondition) {
        // The canonical blob was deleted between Create and Finalize; upload now.
        const retry = await client.createBookUpload({
          filename: file.name,
          contentType: file.type || 'application/octet-stream',
          size: BigInt(file.size)
          // No checksum: force a fresh upload URL.
        })
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
        const retryResult = await client.finalizeBookUpload({
          uploadId: retry.uploadId,
          filename: file.name,
          contentType: file.type || 'application/octet-stream',
          checksum
        })
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

export function useClearLibrary() {
  const client = createServiceClient(CatalogService)
  return () => client.clearLibrary({})
}

export function useStartResync() {
  const client = createServiceClient(CatalogService)
  return () => client.startResync({})
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

// useBookSources live-fetches one book's candidate sources for the book-page
// admin sync control. enabled gates the fetch behind a user action (the
// live fetch hits every configured provider, so it shouldn't run on mount).
export function useBookSources(bookId: string, enabled: boolean) {
  const client = createServiceClient(CatalogService)
  return useSWR<GetBookSourcesResponse, Error>(enabled ? swrKeys.bookSources(bookId) : null, () =>
    client.getBookSources({ bookId })
  )
}

export function useApplyBookSource() {
  const client = useMemo(() => createServiceClient(CatalogService), [])
  return useCallback(
    (bookId: string, source: string) => client.applyBookSource({ bookId, source }),
    [client]
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
