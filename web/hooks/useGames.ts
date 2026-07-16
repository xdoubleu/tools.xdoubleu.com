import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { getApiUrl } from '@/lib/env'
import { swrKeys } from '@/lib/swrKeys'
import { GamesService } from '@/lib/gen/games/v1/games_pb'
import type {
  GetSteamResponse,
  GetSteamGameResponse,
  GetSteamDistributionResponse,
  GetRecentlyActiveGamesResponse,
  GetIntegrationsResponse,
  Integrations
} from '@/lib/gen/games/v1/games_pb'

export function useSteam(fallbackData?: GetSteamResponse) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>(swrKeys.games, () => client.getSteam({}), {
    fallbackData
  })
}

export function useSteamGame(gameId: number, fallbackData?: GetSteamGameResponse) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamGameResponse, Error>(
    gameId ? swrKeys.game(gameId) : null,
    () => client.getSteamGame({ gameId }),
    { fallbackData }
  )
}

export function useSteamDistribution(bucket: number, fallbackData?: GetSteamDistributionResponse) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamDistributionResponse, Error>(
    swrKeys.gamesDistribution(bucket),
    () => client.getSteamDistribution({ bucket }),
    { fallbackData }
  )
}

export function useSteamProgress(dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>(swrKeys.gamesProgress(dateStart, dateEnd), () =>
    client.getSteam({ dateStart, dateEnd })
  )
}

export function useRecentlyActiveGames(fallbackData?: GetRecentlyActiveGamesResponse) {
  const client = createServiceClient(GamesService)
  return useSWR<GetRecentlyActiveGamesResponse, Error>(
    swrKeys.gamesRecent,
    () => client.getRecentlyActiveGames({}),
    { fallbackData }
  )
}

export function useSetGameFavourite() {
  const client = createServiceClient(GamesService)
  return (gameId: number, favourite: boolean) => client.setGameFavourite({ gameId, favourite })
}

export function useRefreshSteamGame() {
  const client = createServiceClient(GamesService)
  return (gameId: number) => client.refreshSteamGame({ gameId })
}

export function useRefreshSteam() {
  return () =>
    fetch(`${getApiUrl()}/games/api/progress/steam/refresh`, {
      credentials: 'include'
    })
}

export function useIntegrations(fallbackData?: GetIntegrationsResponse) {
  const client = createServiceClient(GamesService)
  return useSWR<GetIntegrationsResponse, Error>(
    swrKeys.gamesIntegrations,
    () => client.getIntegrations({}),
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
      fallbackData
    }
  )
}

export function useSaveIntegrations() {
  const client = createServiceClient(GamesService)
  return (integrations: Integrations) => client.saveIntegrations({ integrations })
}
