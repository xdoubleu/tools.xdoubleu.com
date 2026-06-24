import { useRefreshSteam } from '@/hooks/useBacklog'
import { useProgressSocket } from './progressSocket'

export interface SteamRefreshState {
  connected: boolean
  isRefreshing: boolean
  lastRefresh: Date | null
  refresh: () => void
}

// useSteamRefresh subscribes to the backlog progress WebSocket for the "steam"
// topic, mirroring the original pre-migration refresh widget: it reports the
// live refreshing state and last refresh time, exposes a trigger that kicks off
// a server-side Steam sync, and invokes onSynced once a sync completes so the
// caller can re-fetch the freshly synced data.
export function useSteamRefresh(onSynced?: () => void): SteamRefreshState {
  const triggerRefresh = useRefreshSteam()
  const { connected, isRefreshing, lastRefresh, refresh } = useProgressSocket(
    'steam',
    triggerRefresh,
    onSynced
  )
  return { connected, isRefreshing, lastRefresh, refresh }
}
