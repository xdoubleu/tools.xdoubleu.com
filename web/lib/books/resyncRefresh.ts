import { useStartResync } from '@/hooks/useBooks'
import { useProgressSocket, type ProgressState } from '@/lib/progressSocket'

export type ResyncRefreshState = ProgressState

// useResyncRefresh subscribes to the books progress WebSocket for the
// "resync-openlibrary" topic. It exposes:
//   - isRefreshing: true while the scan is running
//   - processed / total: live "X of N" counts (null until first update)
//   - quotaReached: true once Google Books' daily quota trips during the run
//   - lastRefresh: timestamp of the last completed run
//   - refresh(): trigger a new scan
//
// The scan is read-only — it only flags books that differ from an external
// source for the wizard to review; nothing is written to a book here.
// onSynced is called once per completed run so the caller can re-fetch the
// flagged proposals. force bypasses every source's skip-if-known cache for
// this run — see BookService.BuildResyncProposals.
export function useResyncRefresh(onSynced?: () => void, force = false): ResyncRefreshState {
  const triggerResync = useStartResync()
  return useProgressSocket('books', 'resync-openlibrary', () => triggerResync(force), onSynced)
}
