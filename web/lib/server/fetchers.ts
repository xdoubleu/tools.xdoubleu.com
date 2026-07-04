import { ConnectError } from '@connectrpc/connect'

// fetchOrNull makes server-side prefetching strictly best-effort: any API
// rejection (expired access token awaiting browser-side refresh, missing
// permissions, transient upstream failure) returns null so the page renders
// and the client component's own SWR fetch takes over — exactly the pre-RSC
// behavior. Non-Connect errors are real bugs and propagate to app/error.tsx.
export async function fetchOrNull<T>(fn: () => Promise<T>): Promise<T | null> {
  try {
    return await fn()
  } catch (err) {
    if (err instanceof ConnectError) {
      return null
    }
    throw err
  }
}
