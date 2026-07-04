import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { getApiUrl } from '@/lib/env'
import { GamesService } from '@/lib/gen/games/v1/games_pb'
import type {
  GetSteamResponse,
  GetSteamGameResponse,
  GetSteamDistributionResponse,
  GetRecentlyActiveGamesResponse,
  GetIntegrationsResponse,
  Integrations
} from '@/lib/gen/games/v1/games_pb'

export function useSteam() {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>('/games', () => client.getSteam({}))
}

export function useSteamGame(gameId: number) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamGameResponse, Error>(gameId ? `/games/${gameId}` : null, () =>
    client.getSteamGame({ gameId })
  )
}

export function useSteamDistribution(bucket: number) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamDistributionResponse, Error>(`/games/distribution/${bucket}`, () =>
    client.getSteamDistribution({ bucket })
  )
}

export function useSteamProgress(dateStart?: string, dateEnd?: string) {
  const client = createServiceClient(GamesService)
  return useSWR<GetSteamResponse, Error>(['/games/progress', dateStart, dateEnd], () =>
    client.getSteam({ dateStart, dateEnd })
  )
}

export function useRecentlyActiveGames() {
  const client = createServiceClient(GamesService)
  return useSWR<GetRecentlyActiveGamesResponse, Error>('/games/recent', () =>
    client.getRecentlyActiveGames({})
  )
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

export function useIntegrations() {
  const client = createServiceClient(GamesService)
  return useSWR<GetIntegrationsResponse, Error>(
    '/games/integrations',
    () => client.getIntegrations({}),
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false
    }
  )
}

export function useSaveIntegrations() {
  const client = createServiceClient(GamesService)
  return (integrations: Integrations) => client.saveIntegrations({ integrations })
}
