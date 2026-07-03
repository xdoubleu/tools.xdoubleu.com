import { useRefreshSteam } from '@/hooks/useGames'
import { useProgressSocket } from '@/lib/progressSocket'

export interface SteamRefreshState {
  connected: boolean
  isRefreshing: boolean
  lastRefresh: Date | null
  refresh: () => void
}

// useSteamRefresh subscribes to the games progress WebSocket for the "steam"
// topic, mirroring the original pre-migration refresh widget: it reports the
// live refreshing state and last refresh time, exposes a trigger that kicks off
// a server-side Steam sync, and invokes onSynced once a sync completes so the
// caller can re-fetch the freshly synced data.
export function useSteamRefresh(onSynced?: () => void): SteamRefreshState {
  const triggerRefresh = useRefreshSteam()
  const { connected, isRefreshing, lastRefresh, refresh } = useProgressSocket(
    'games',
    'steam',
    triggerRefresh,
    onSynced
  )
  return { connected, isRefreshing, lastRefresh, refresh }
}
