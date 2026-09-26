import posthog from 'posthog-js'

/** Captures a custom PostHog event; a no-op until `instrumentation-client.ts` has initialised PostHog. */
export function track(event: string, properties?: Record<string, unknown>): void {
  if (!posthog.__loaded) return
  posthog.capture(event, properties)
}
