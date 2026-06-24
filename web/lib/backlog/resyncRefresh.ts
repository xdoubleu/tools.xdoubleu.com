import { useResyncOpenLibrary } from '@/hooks/useBacklog'
import { useProgressSocket, type ProgressState } from './progressSocket'

export type ResyncRefreshState = ProgressState

// useResyncRefresh subscribes to the backlog progress WebSocket for the
// "resync-openlibrary" topic. It exposes:
//   - isRefreshing: true while the job is running
//   - processed / total: live "X of N books" counts (null until first update)
//   - lastRefresh: timestamp of the last completed run
//   - refresh(): trigger a new resync
//
// onSynced is called once per completed run so the caller can re-fetch data.
export function useResyncRefresh(onSynced?: () => void): ResyncRefreshState {
  const triggerResync = useResyncOpenLibrary()
  return useProgressSocket('resync-openlibrary', triggerResync, onSynced)
}
