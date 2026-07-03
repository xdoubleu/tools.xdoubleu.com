import { useResyncOpenLibrary } from '@/hooks/useBooks'
import { useProgressSocket, type ProgressState } from '@/lib/progressSocket'

export type ResyncRefreshState = ProgressState

// useResyncRefresh subscribes to the books progress WebSocket for the
// "resync-openlibrary" topic. It exposes:
//   - isRefreshing: true while the job is running
//   - processed / total: live "X of N books" counts (null until first update)
//   - lastRefresh: timestamp of the last completed run
//   - refresh(): trigger a new resync
//
// onSynced is called once per completed run so the caller can re-fetch data.
export function useResyncRefresh(onSynced?: () => void): ResyncRefreshState {
  const triggerResync = useResyncOpenLibrary()
  return useProgressSocket('books', 'resync-openlibrary', triggerResync, onSynced)
}
