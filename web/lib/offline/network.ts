import { Code, ConnectError } from '@connectrpc/connect'

/** True when the request never reached the API (offline, DNS, proxy down). */
export function isNetworkError(err: unknown): boolean {
  if (typeof navigator !== 'undefined' && !navigator.onLine) return true
  if (err instanceof TypeError) return true
  if (err instanceof ConnectError) {
    return err.code === Code.Unavailable || err.cause instanceof TypeError
  }
  return false
}
