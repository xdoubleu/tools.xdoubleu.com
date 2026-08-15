import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { swrKeys } from '@/lib/swrKeys'
import { DashboardKind, DashboardService } from '@/lib/gen/dashboard/v1/dashboard_pb'
import type { GetDashboardShareResponse } from '@/lib/gen/dashboard/v1/dashboard_pb'
import { PublicReadingDashboardService } from '@/lib/gen/dashboard/v1/reading_pb'
import type {
  GetSharedLibraryResponse,
  GetSharedBooksProgressResponse,
  GetSharedFeedsSummaryResponse
} from '@/lib/gen/dashboard/v1/reading_pb'
import { PublicGamesDashboardService } from '@/lib/gen/dashboard/v1/games_pb'
import type {
  GetSharedSteamResponse,
  GetSharedSteamGameResponse,
  GetSharedRecentlyActiveGamesResponse
} from '@/lib/gen/dashboard/v1/games_pb'

// Owner-side share management. Each dashboard (games, reading) has its own
// independent share link.

export type DashboardShareKind = 'games' | 'reading'

const DASHBOARD_KIND_ENUM: Record<DashboardShareKind, DashboardKind> = {
  games: DashboardKind.GAMES,
  reading: DashboardKind.READING
}

export function useDashboardShare(
  kind: DashboardShareKind,
  fallbackData?: GetDashboardShareResponse
) {
  const client = createServiceClient(DashboardService)
  return useSWR<GetDashboardShareResponse, Error>(
    swrKeys.dashboardShare(kind),
    () => client.getDashboardShare({ kind: DASHBOARD_KIND_ENUM[kind] }),
    { fallbackData }
  )
}

export function useCreateDashboardShare(kind: DashboardShareKind) {
  const client = createServiceClient(DashboardService)
  return () => client.createDashboardShare({ kind: DASHBOARD_KIND_ENUM[kind] })
}

export function useDeleteDashboardShare(kind: DashboardShareKind) {
  const client = createServiceClient(DashboardService)
  return () => client.deleteDashboardShare({ kind: DASHBOARD_KIND_ENUM[kind] })
}

// Public (token-gated, no session) dashboard data.

export function useSharedLibrary(token: string, fallbackData?: GetSharedLibraryResponse) {
  const client = createServiceClient(PublicReadingDashboardService)
  return useSWR<GetSharedLibraryResponse, Error>(
    token ? swrKeys.dashboardReading(token) : null,
    () => client.getSharedLibrary({ token }),
    { fallbackData }
  )
}

export function useSharedBooksProgress(token: string, dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(PublicReadingDashboardService)
  return useSWR<GetSharedBooksProgressResponse, Error>(
    token ? swrKeys.dashboardReadingProgress(token, dateStart, dateEnd) : null,
    () => client.getSharedBooksProgress({ token, dateStart, dateEnd })
  )
}

export function useSharedFeedsSummary(token: string, fallbackData?: GetSharedFeedsSummaryResponse) {
  const client = createServiceClient(PublicReadingDashboardService)
  return useSWR<GetSharedFeedsSummaryResponse, Error>(
    token ? swrKeys.dashboardFeedsSummary(token) : null,
    () => client.getSharedFeedsSummary({ token }),
    { fallbackData }
  )
}

export function useSharedSteam(token: string, fallbackData?: GetSharedSteamResponse) {
  const client = createServiceClient(PublicGamesDashboardService)
  return useSWR<GetSharedSteamResponse, Error>(
    token ? swrKeys.dashboardGames(token) : null,
    () => client.getSharedSteam({ token }),
    { fallbackData }
  )
}

export function useSharedSteamProgress(token: string, dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(PublicGamesDashboardService)
  return useSWR<GetSharedSteamResponse, Error>(
    token ? swrKeys.dashboardGamesProgress(token, dateStart, dateEnd) : null,
    () => client.getSharedSteam({ token, dateStart, dateEnd })
  )
}

export function useSharedSteamGame(
  token: string,
  gameId: number,
  fallbackData?: GetSharedSteamGameResponse
) {
  const client = createServiceClient(PublicGamesDashboardService)
  return useSWR<GetSharedSteamGameResponse, Error>(
    token && gameId ? swrKeys.dashboardGame(token, gameId) : null,
    () => client.getSharedSteamGame({ token, gameId }),
    { fallbackData }
  )
}

export function useSharedRecentlyActiveGames(
  token: string,
  fallbackData?: GetSharedRecentlyActiveGamesResponse
) {
  const client = createServiceClient(PublicGamesDashboardService)
  return useSWR<GetSharedRecentlyActiveGamesResponse, Error>(
    token ? swrKeys.dashboardRecentGames(token) : null,
    () => client.getSharedRecentlyActiveGames({ token }),
    { fallbackData }
  )
}
