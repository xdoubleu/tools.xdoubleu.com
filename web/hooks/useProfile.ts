import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { swrKeys } from '@/lib/swrKeys'
import { ProfileService } from '@/lib/gen/profile/v1/profile_pb'
import type { GetProfileShareResponse } from '@/lib/gen/profile/v1/profile_pb'
import { PublicLibraryService } from '@/lib/gen/books/v1/public_pb'
import type {
  GetSharedLibraryResponse,
  GetSharedBooksProgressResponse
} from '@/lib/gen/books/v1/public_pb'
import { PublicGamesService } from '@/lib/gen/games/v1/public_pb'
import type {
  GetSharedSteamResponse,
  GetSharedSteamGameResponse,
  GetSharedRecentlyActiveGamesResponse
} from '@/lib/gen/games/v1/public_pb'

// Owner-side share management.

export function useProfileShare(fallbackData?: GetProfileShareResponse) {
  const client = createServiceClient(ProfileService)
  return useSWR<GetProfileShareResponse, Error>(
    swrKeys.profileShare,
    () => client.getProfileShare({}),
    { fallbackData }
  )
}

export function useCreateProfileShare() {
  const client = createServiceClient(ProfileService)
  return () => client.createProfileShare({})
}

export function useDeleteProfileShare() {
  const client = createServiceClient(ProfileService)
  return () => client.deleteProfileShare({})
}

// Public (token-gated, no session) profile data.

export function useSharedLibrary(token: string, fallbackData?: GetSharedLibraryResponse) {
  const client = createServiceClient(PublicLibraryService)
  return useSWR<GetSharedLibraryResponse, Error>(
    token ? swrKeys.profileBooks(token) : null,
    () => client.getSharedLibrary({ token }),
    { fallbackData }
  )
}

export function useSharedBooksProgress(token: string, dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(PublicLibraryService)
  return useSWR<GetSharedBooksProgressResponse, Error>(
    token ? swrKeys.profileBooksProgress(token, dateStart, dateEnd) : null,
    () => client.getSharedBooksProgress({ token, dateStart, dateEnd })
  )
}

export function useSharedSteam(token: string, fallbackData?: GetSharedSteamResponse) {
  const client = createServiceClient(PublicGamesService)
  return useSWR<GetSharedSteamResponse, Error>(
    token ? swrKeys.profileGames(token) : null,
    () => client.getSharedSteam({ token }),
    { fallbackData }
  )
}

export function useSharedSteamProgress(token: string, dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(PublicGamesService)
  return useSWR<GetSharedSteamResponse, Error>(
    token ? swrKeys.profileGamesProgress(token, dateStart, dateEnd) : null,
    () => client.getSharedSteam({ token, dateStart, dateEnd })
  )
}

export function useSharedSteamGame(
  token: string,
  gameId: number,
  fallbackData?: GetSharedSteamGameResponse
) {
  const client = createServiceClient(PublicGamesService)
  return useSWR<GetSharedSteamGameResponse, Error>(
    token && gameId ? swrKeys.profileGame(token, gameId) : null,
    () => client.getSharedSteamGame({ token, gameId }),
    { fallbackData }
  )
}

export function useSharedRecentlyActiveGames(
  token: string,
  fallbackData?: GetSharedRecentlyActiveGamesResponse
) {
  const client = createServiceClient(PublicGamesService)
  return useSWR<GetSharedRecentlyActiveGamesResponse, Error>(
    token ? swrKeys.profileRecentGames(token) : null,
    () => client.getSharedRecentlyActiveGames({ token }),
    { fallbackData }
  )
}
