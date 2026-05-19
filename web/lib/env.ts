declare global {
  interface Window {
    __ENV__: { API_URL: string; SENTRY_DSN: string }
  }
}

export function getApiUrl(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.API_URL ?? ''
  return process.env.API_URL ?? ''
}

export function getSentryDsn(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.SENTRY_DSN ?? ''
  return process.env.SENTRY_DSN ?? ''
}
