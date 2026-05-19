import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { BooksService } from '@/lib/gen/backlog/v1/books_connect'
import { GamesService } from '@/lib/gen/backlog/v1/games_connect'
import type {
  GetLibraryResponse,
  GetBooksProgressResponse,
  SearchExternalResponse,
  AddBookRequest,
  UpdateBookStatusRequest,
  ToggleTagRequest
} from '@/lib/gen/backlog/v1/books_pb'
import type {
  GetSteamResponse,
  GetSteamGameResponse,
  GetSteamDistributionResponse
} from '@/lib/gen/backlog/v1/games_pb'

export function useBacklogLibrary() {
  const client = createServiceClient(BooksService)
  return useSWR<GetLibraryResponse, Error>('/backlog/books', () => client.getLibrary({}))
}

export function useBacklogSteam() {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>('/backlog/steam', () => client.getSteam({}))
}

export function useBacklogSteamGame(gameId: number) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamGameResponse, Error>(gameId ? `/backlog/steam/${gameId}` : null, () =>
    client.getSteamGame({ gameId })
  )
}

export function useBacklogDistribution() {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamDistributionResponse, Error>('/backlog/steam/distribution', () =>
    client.getSteamDistribution({})
  )
}

export function useBooksProgress(dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(BooksService)
  const key = dateStart || dateEnd ? ['/backlog/books/progress', dateStart, dateEnd] : null
  return useSWR<GetBooksProgressResponse, Error>(key, () =>
    client.getBooksProgress({ dateStart, dateEnd })
  )
}

export function useSearchExternal() {
  const client = createServiceClient(BooksService)
  return (query: string) => client.searchExternal({ query }).then((r: SearchExternalResponse) => r)
}

export function useAddBook() {
  const client = createServiceClient(BooksService)
  return (req: AddBookRequest) => client.addBook(req)
}

export function useUpdateBookStatus() {
  const client = createServiceClient(BooksService)
  return (req: UpdateBookStatusRequest) => client.updateBookStatus(req)
}

export function useToggleTag() {
  const client = createServiceClient(BooksService)
  return (req: ToggleTagRequest) => client.toggleTag(req)
}

export function useImportBooks() {
  const client = createServiceClient(BooksService)
  return (csvData: string) => {
    const encoder = new TextEncoder()
    return client.importBooks({ csvData: encoder.encode(csvData) })
  }
}
