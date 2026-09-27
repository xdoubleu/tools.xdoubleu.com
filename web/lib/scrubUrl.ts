import type { CaptureResult } from 'posthog-js'

// Query params that carry credentials (the password-reset token) and must not
// reach analytics.
const SENSITIVE_PARAMS = ['token', 'code']

const URL_PROPERTIES = ['$current_url', '$referrer', '$initial_current_url', '$initial_referrer']

export function stripSensitiveParams(url: string): string {
  let parsed: URL
  try {
    parsed = new URL(url)
  } catch {
    return url
  }
  let changed = false
  for (const param of SENSITIVE_PARAMS) {
    if (parsed.searchParams.has(param)) {
      parsed.searchParams.set(param, 'redacted')
      changed = true
    }
  }
  return changed ? parsed.toString() : url
}

// PostHog before_send: redacts credential query params from URL properties.
export function scrubPostHogEvent(event: CaptureResult | null): CaptureResult | null {
  if (!event) return event
  for (const key of URL_PROPERTIES) {
    const value = event.properties[key]
    if (typeof value === 'string') event.properties[key] = stripSensitiveParams(value)
  }
  return event
}
