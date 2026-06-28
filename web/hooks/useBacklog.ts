import { useCallback, useMemo } from 'react'
import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { ConnectError, Code } from '@connectrpc/connect'
import { createServiceClient } from '@/lib/client'
import { getApiUrl } from '@/lib/env'
import { sha256Hex } from '@/lib/backlog/checksum'
import {
  BooksService,
  AddBookRequestSchema,
  UpdateBookStatusRequestSchema,
  UpdateProgressRequestSchema
} from '@/lib/gen/backlog/v1/books_pb'
import { GamesService } from '@/lib/gen/backlog/v1/games_pb'
import type {
  GetLibraryResponse,
  GetBooksProgressResponse,
  SearchLibraryResponse,
  SearchExternalResponse,
  GetKEPUBStatusResponse,
  GetBookFileResponse,
  ListKoboDevicesResponse,
  FindDuplicatesResponse,
  ListCatalogBooksResponse,
  Book
} from '@/lib/gen/backlog/v1/books_pb'

export type AddBookInput = MessageInitShape<typeof AddBookRequestSchema>
export type UpdateBookStatusInput = MessageInitShape<typeof UpdateBookStatusRequestSchema>
export type UpdateProgressInput = MessageInitShape<typeof UpdateProgressRequestSchema>
import type {
  GetSteamResponse,
  GetSteamGameResponse,
  GetSteamDistributionResponse,
  GetRecentlyActiveGamesResponse
} from '@/lib/gen/backlog/v1/games_pb'

export function useBacklogLibrary() {
  const client = createServiceClient(BooksService)
  return useSWR<GetLibraryResponse, Error>('/backlog/books', () => client.getLibrary({}))
}

export function useBacklogSteam() {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>('/backlog/games', () => client.getSteam({}))
}

export function useBacklogSteamGame(gameId: number) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamGameResponse, Error>(gameId ? `/backlog/games/${gameId}` : null, () =>
    client.getSteamGame({ gameId })
  )
}

export function useBacklogDistribution(bucket: number) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamDistributionResponse, Error>(`/backlog/games/distribution/${bucket}`, () =>
    client.getSteamDistribution({ bucket })
  )
}

export function useSteamProgress(dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>(['/backlog/games/progress', dateStart, dateEnd], () =>
    client.getSteam({ dateStart, dateEnd })
  )
}

export function useRecentlyActiveGames() {
  const client = createServiceClient(GamesService)
  return useSWR<GetRecentlyActiveGamesResponse, Error>('/backlog/games/recent', () =>
    client.getRecentlyActiveGames({})
  )
}

export function useBooksProgress(dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(BooksService)
  const key = dateStart || dateEnd ? ['/backlog/books/progress', dateStart, dateEnd] : null
  return useSWR<GetBooksProgressResponse, Error>(key, () =>
    client.getBooksProgress({ dateStart, dateEnd })
  )
}

export function useRefreshSteamGame() {
  const client = createServiceClient(GamesService)
  return (gameId: number) => client.refreshSteamGame({ gameId })
}

export function useRefreshSteam() {
  return () =>
    fetch(`${getApiUrl()}/backlog/api/progress/steam/refresh`, {
      credentials: 'include'
    })
}

export function useSearchLibrary() {
  const client = useMemo(() => createServiceClient(BooksService), [])
  return useCallback(
    (query: string) => client.searchLibrary({ query }).then((r: SearchLibraryResponse) => r),
    [client]
  )
}

export function useSearchExternal() {
  const client = useMemo(() => createServiceClient(BooksService), [])
  return useCallback(
    (query: string) => client.searchExternal({ query }).then((r: SearchExternalResponse) => r),
    [client]
  )
}

export function useAddBook() {
  const client = createServiceClient(BooksService)
  return (req: AddBookInput) => client.addBook(req)
}

export function useImportBooks() {
  const client = createServiceClient(BooksService)
  return (csvData: string) => {
    const encoder = new TextEncoder()
    return client.importBooks({ csvData: encoder.encode(csvData) })
  }
}

export function useUpdateBookStatus() {
  const client = createServiceClient(BooksService)
  return (req: UpdateBookStatusInput) => client.updateBookStatus(req)
}

export function useToggleTag() {
  const client = createServiceClient(BooksService)
  return (bookId: string, tag: string) => client.toggleTag({ bookId, tag })
}

export function useUpdateProgress() {
  const client = createServiceClient(BooksService)
  return (req: UpdateProgressInput) => client.updateProgress(req)
}

export type UploadBookFileResult = {
  matchedExisting: boolean
  recognizedTitle: string
}

export function useUploadBookFile() {
  const client = createServiceClient(BooksService)
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
  const client = createServiceClient(BooksService)
  return (bookId: string) => client.enableKoboSync({ bookId })
}

export function useRequestKEPUBConversion() {
  const client = createServiceClient(BooksService)
  return (bookId: string) => client.requestKEPUBConversion({ bookId })
}

export function useRegisterKoboDevice() {
  const client = createServiceClient(BooksService)
  return (name: string, serial: string) => client.registerKoboDevice({ name, serial })
}

export function useListKoboDevices() {
  const client = createServiceClient(BooksService)
  return useSWR<ListKoboDevicesResponse, Error>('/backlog/kobo/devices', () =>
    client.listKoboDevices({})
  )
}

export function useDisconnectKoboDevice() {
  const client = createServiceClient(BooksService)
  return (id: string) => client.disconnectKoboDevice({ id })
}

export function useClearLibrary() {
  const client = createServiceClient(BooksService)
  return () => client.clearLibrary({})
}

export function useResyncOpenLibrary() {
  const client = createServiceClient(BooksService)
  return () => client.resyncOpenLibrary({})
}

export function useFindDuplicates() {
  const client = createServiceClient(BooksService)
  return useSWR<FindDuplicatesResponse, Error>('/backlog/books/duplicates', () =>
    client.findDuplicates({})
  )
}

export interface MergeBooksOptions {
  resolvedMetadata?: Book
  resolvedCoverSourceBookId?: string
  resolvedStatus?: string
}

export function useMergeBooks() {
  const client = createServiceClient(BooksService)
  return (winnerBookId: string, loserBookIds: string[], options?: MergeBooksOptions) =>
    client.mergeBooks({
      winnerBookId,
      loserBookIds,
      resolvedMetadata: options?.resolvedMetadata,
      resolvedCoverSourceBookId: options?.resolvedCoverSourceBookId,
      resolvedStatus: options?.resolvedStatus
    })
}

export function useCatalogBooks() {
  const client = createServiceClient(BooksService)
  return useSWR<ListCatalogBooksResponse, Error>('/backlog/books/catalog', () =>
    client.listCatalogBooks({})
  )
}

export function useResyncBooks() {
  const client = useMemo(() => createServiceClient(BooksService), [])
  return useCallback(
    (bookIds: string[], force: boolean) => client.resyncBooks({ bookIds, force }),
    [client]
  )
}

export function useSetBookISBN() {
  const client = useMemo(() => createServiceClient(BooksService), [])
  return useCallback(
    (bookId: string, isbn13: string) => client.setBookISBN({ bookId, isbn13 }),
    [client]
  )
}

export function useKEPUBStatus(bookId: string | null) {
  const client = createServiceClient(BooksService)
  return useSWR<GetKEPUBStatusResponse, Error>(
    bookId ? ['/backlog/books/kepub-status', bookId] : null,
    () => client.getKEPUBStatus({ bookId: bookId! }),
    { refreshInterval: (data) => (data?.kepubStatus === 'converting' ? 2000 : 0) }
  )
}

export function useGetBookFile(bookId: string | null, format: string | null) {
  const client = createServiceClient(BooksService)
  return useSWR<GetBookFileResponse, Error>(
    bookId && format ? ['/backlog/books/file', bookId, format] : null,
    () => client.getBookFile({ bookId: bookId!, format: format! })
  )
}

export function useRenameShelf() {
  const client = createServiceClient(BooksService)
  return (oldName: string, newName: string) => client.renameShelf({ oldName, newName })
}

export function useDeleteShelf() {
  const client = createServiceClient(BooksService)
  return (name: string, targetName: string) => client.deleteShelf({ name, targetName })
}

export function useRenameTag() {
  const client = createServiceClient(BooksService)
  return (oldName: string, newName: string) => client.renameTag({ oldName, newName })
}

export function useDeleteTag() {
  const client = createServiceClient(BooksService)
  return (name: string) => client.deleteTag({ name })
}
