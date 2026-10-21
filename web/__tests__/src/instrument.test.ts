const { mockSentryInit, mockPostHogInit } = vi.hoisted(() => ({
  mockSentryInit: vi.fn(),
  mockPostHogInit: vi.fn()
}))

vi.mock('@sentry/react', () => ({ init: mockSentryInit }))

vi.mock('@sentry/react/react-router', () => ({
  reactRouterBrowserTracingIntegration: vi.fn(() => ({ name: 'BrowserTracing' }))
}))

vi.mock('posthog-js', () => ({
  default: { init: mockPostHogInit }
}))

vi.mock('@/lib/env', () => ({
  getSentryDsn: vi.fn(() => 'https://example@sentry.io/123'),
  getRelease: vi.fn(() => 'v1.0.0'),
  getPostHogKey: vi.fn(),
  getPostHogHost: vi.fn(() => 'https://eu.i.posthog.com')
}))

import { getPostHogKey } from '@/lib/env'

describe('instrument', () => {
  beforeEach(() => {
    vi.resetModules()
    mockSentryInit.mockClear()
    mockPostHogInit.mockClear()
  })

  it('always initializes Sentry', async () => {
    await import('@/src/instrument')

    expect(mockSentryInit).toHaveBeenCalledWith(
      expect.objectContaining({ dsn: 'https://example@sentry.io/123', release: 'v1.0.0' })
    )
  })

  it('names transactions by route', async () => {
    await import('@/src/instrument')

    expect(mockSentryInit).toHaveBeenCalledWith(
      expect.objectContaining({ integrations: [{ name: 'BrowserTracing' }] })
    )
  })

  it('initializes PostHog when a key is configured', async () => {
    vi.mocked(getPostHogKey).mockReturnValue('phc_abc123')

    await import('@/src/instrument')

    expect(mockPostHogInit).toHaveBeenCalledWith('phc_abc123', {
      api_host: 'https://eu.i.posthog.com',
      person_profiles: 'identified_only',
      capture_pageview: 'history_change',
      capture_pageleave: true,
      autocapture: true,
      capture_dead_clicks: true,
      before_send: expect.any(Function)
    })
  })

  it('skips PostHog initialization when no key is configured', async () => {
    vi.mocked(getPostHogKey).mockReturnValue('')

    await import('@/src/instrument')

    expect(mockPostHogInit).not.toHaveBeenCalled()
  })
})
