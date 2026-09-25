import { useRefreshSteam } from '@/hooks/useGames'
import { useProgressSocket } from '@/hooks/useProgressSocket'

export interface SteamRefreshState {
  connected: boolean
  isRefreshing: boolean
  lastRefresh: Date | null
  refresh: () => void
}

// useSteamRefresh tracks the "steam" topic on the games progress WebSocket,
// triggers a server-side sync, and calls onSynced when it completes.
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
