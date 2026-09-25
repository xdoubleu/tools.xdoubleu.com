import { useStartResync } from '@/hooks/useBooks'
import { useProgressSocket, type ProgressState } from '@/hooks/useProgressSocket'

export type ResyncRefreshState = ProgressState

// useResyncRefresh tracks the read-only "resync-books" scan over the progress
// WebSocket (it only flags books for review). onSynced fires per completed
// run; force bypasses every source's skip-if-known cache.
export function useResyncRefresh(onSynced?: () => void, force = false): ResyncRefreshState {
  const triggerResync = useStartResync()
  return useProgressSocket('books', 'resync-books', () => triggerResync(force), onSynced)
}
