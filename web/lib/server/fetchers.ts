import { Code, ConnectError } from '@connectrpc/connect'
import { captureException } from '@sentry/nextjs'

// Expected during normal degradation, so not reported: an expired access token
// (401 here, recovered by the browser's SWR refresh), missing app access, or
// the api's own rate limiter correctly throttling a client IP. That last one
// fires on every SSR render (including Next's default /_not-found, which
// still runs through the root layout's current-user prefetch), so a burst of
// bot/scanner traffic hitting made-up routes trips it repeatedly — the
// limiter doing its job, not a bug to surface as an error (issue #1318).
const EXPECTED_CODES = new Set([
  Code.Unauthenticated,
  Code.PermissionDenied,
  Code.ResourceExhausted
])

// fetchOrNull makes server-side prefetching strictly best-effort: any API
// rejection (expired access token awaiting browser-side refresh, missing
// permissions, transient upstream failure) returns null so the page renders
// and the client component's own SWR fetch takes over — exactly the pre-RSC
// behavior. Non-Connect errors are real bugs and propagate to app/error.tsx.
//
// Unexpected ConnectErrors (DeadlineExceeded from a hung/slow api, Unavailable,
// Internal) are still swallowed to null but reported to Sentry first — a hung
// api used to leave no trace at all (see issue #634).
export async function fetchOrNull<T>(fn: () => Promise<T>): Promise<T | null> {
  try {
    return await fn()
  } catch (err) {
    if (err instanceof ConnectError) {
      if (!EXPECTED_CODES.has(err.code) && !isTransientNetworkFailure(err)) {
        captureException(err)
      }
      return null
    }
    throw err
  }
}

// connect-web wraps a raw fetch failure (e.g. Safari's "TypeError: Load
// failed" on a dropped connection) into a ConnectError with code Unknown and
// the original TypeError as `cause` — there was never a server response to
// signal anything with (unlike a real Unknown from the server, or
// DeadlineExceeded/Unavailable). Treat it the same as the expected codes:
// nothing to report, the page's null fallback already covers it.
function isTransientNetworkFailure(err: ConnectError): boolean {
  return err.code === Code.Unknown && err.cause instanceof TypeError
}
