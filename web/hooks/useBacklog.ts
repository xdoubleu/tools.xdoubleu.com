import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { BooksService } from '@/lib/gen/backlog/v1/books_connect'
import { GamesService } from '@/lib/gen/backlog/v1/games_connect'
import type { GetLibraryResponse } from '@/lib/gen/backlog/v1/books_pb'
import type { GetSteamResponse } from '@/lib/gen/backlog/v1/games_pb'

export function useBacklogLibrary() {
  const client = createServiceClient(BooksService)
  return useSWR<GetLibraryResponse, Error>('/backlog/books', () => client.getLibrary({}))
}

export function useBacklogSteam() {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>('/backlog/steam', () => client.getSteam({}))
}
