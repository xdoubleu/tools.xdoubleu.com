import { Code, ConnectError } from '@connectrpc/connect'
import { captureException } from '@sentry/nextjs'

// Expected degradation, not reported: expired tokens (the browser refreshes),
// missing app access, and the api's rate limiter (which bot traffic trips on
// every SSR render).
const EXPECTED_CODES = new Set([
  Code.Unauthenticated,
  Code.PermissionDenied,
  Code.ResourceExhausted
])

// fetchOrNull makes server prefetching best-effort: any ConnectError returns
// null and the client's SWR fetch takes over. Unexpected codes are reported
// to Sentry first; non-Connect errors propagate to app/error.tsx.
export async function fetchOrNull<T>(fn: () => Promise<T>): Promise<T | null> {
  try {
    return await fn()
  } catch (err) {
    if (err instanceof ConnectError) {
      if (
        !EXPECTED_CODES.has(err.code) &&
        !isTransientNetworkFailure(err) &&
        !isRawHTTPNotFound(err)
      ) {
        captureException(err)
      }
      return null
    }
    throw err
  }
}

// A raw fetch failure wrapped by connect-web as Unknown with a TypeError
// cause: no server response, nothing to report.
function isTransientNetworkFailure(err: ConnectError): boolean {
  return err.code === Code.Unknown && err.cause instanceof TypeError
}

// A plain "HTTP 404" Unimplemented comes from connect-go for an unknown
// procedure: the window between web's and api's redeploys (ADR 0001), not a
// bug. Real Connect errors carry the server's own message.
function isRawHTTPNotFound(err: ConnectError): boolean {
  return err.code === Code.Unimplemented && err.rawMessage === 'HTTP 404'
}
