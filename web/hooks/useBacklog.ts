import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import { getApiUrl } from '@/lib/env'
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
  SearchExternalResponse
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

export function useSearchExternal() {
  const client = createServiceClient(BooksService)
  return (query: string) => client.searchExternal({ query }).then((r: SearchExternalResponse) => r)
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
